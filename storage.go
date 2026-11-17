package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type Reminder struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Text      string    `json:"text"`
	DueTime   time.Time `json:"due_time"`
	SoundType string    `json:"sound_type"`
	Completed bool      `json:"completed"`
	Triggered bool      `json:"triggered"` // Whether alarm has fired
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	mu           sync.RWMutex
	filePath     string
	reminders    map[string]*Reminder
	activeAlarms map[string]*Reminder
}

func generateID() string {
	bytes := make([]byte, 6)
	rand.Read(bytes)
	return fmt.Sprintf("rem_%d_%s", time.Now().Unix(), hex.EncodeToString(bytes))
}

func NewStore(filename string) (*Store, error) {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		absPath = filename
	}

	s := &Store{
		filePath:     absPath,
		reminders:    make(map[string]*Reminder),
		activeAlarms: make(map[string]*Reminder),
	}

	if err := s.load(); err != nil {
		// If file doesn't exist yet, start empty
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var list []*Reminder
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.reminders = make(map[string]*Reminder)
	for _, r := range list {
		s.reminders[r.ID] = r
	}
	return nil
}

func (s *Store) saveLocked() error {
	list := make([]*Reminder, 0, len(s.reminders))
	for _, r := range s.reminders {
		list = append(list, r)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *Store) GetAll() []*Reminder {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Reminder, 0, len(s.reminders))
	for _, r := range s.reminders {
		// Make a copy to avoid race conditions
		cp := *r
		list = append(list, &cp)
	}

	// Sort by DueTime ascending
	sort.Slice(list, func(i, j int) bool {
		return list[i].DueTime.Before(list[j].DueTime)
	})

	return list
}

func (s *Store) Get(id string) (*Reminder, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.reminders[id]
	if !ok {
		return nil, false
	}
	cp := *r
	return &cp, true
}

func (s *Store) Create(title, text string, dueTime time.Time, soundType string) (*Reminder, error) {
	if title == "" {
		title = "Recordatorio sin título"
	}
	if soundType == "" {
		soundType = "campana"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	r := &Reminder{
		ID:        generateID(),
		Title:     title,
		Text:      text,
		DueTime:   dueTime,
		SoundType: soundType,
		Completed: false,
		Triggered: false,
		CreatedAt: time.Now(),
	}

	s.reminders[r.ID] = r
	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	cp := *r
	return &cp, nil
}

func (s *Store) Update(id, title, text string, dueTime time.Time, soundType string) (*Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.reminders[id]
	if !ok {
		return nil, fmt.Errorf("recordatorio no encontrado")
	}

	if title != "" {
		r.Title = title
	}
	r.Text = text
	if !dueTime.IsZero() {
		// If due time changed, reset triggered status
		if !dueTime.Equal(r.DueTime) {
			r.Triggered = false
			delete(s.activeAlarms, id)
		}
		r.DueTime = dueTime
	}
	if soundType != "" {
		r.SoundType = soundType
	}

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	cp := *r
	return &cp, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.reminders[id]; !ok {
		return fmt.Errorf("recordatorio no encontrado")
	}

	delete(s.reminders, id)
	delete(s.activeAlarms, id)
	return s.saveLocked()
}

func (s *Store) ToggleComplete(id string) (*Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.reminders[id]
	if !ok {
		return nil, fmt.Errorf("recordatorio no encontrado")
	}

	r.Completed = !r.Completed
	if r.Completed {
		delete(s.activeAlarms, id)
	}

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	cp := *r
	return &cp, nil
}

func (s *Store) Snooze(id string, minutes int) (*Reminder, error) {
	if minutes <= 0 {
		minutes = 5
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.reminders[id]
	if !ok {
		return nil, fmt.Errorf("recordatorio no encontrado")
	}

	r.DueTime = time.Now().Add(time.Duration(minutes) * time.Minute)
	r.Triggered = false
	r.Completed = false
	delete(s.activeAlarms, id)

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	cp := *r
	return &cp, nil
}

func (s *Store) DismissAlarm(id string) (*Reminder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.activeAlarms, id)

	r, ok := s.reminders[id]
	if !ok {
		return nil, fmt.Errorf("recordatorio no encontrado")
	}

	cp := *r
	return &cp, nil
}

// CheckDue inspects reminders and triggers any that are past their due time
func (s *Store) CheckDue() []*Reminder {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var newlyDue []*Reminder

	for _, r := range s.reminders {
		if !r.Completed && !r.Triggered && (now.After(r.DueTime) || now.Equal(r.DueTime)) {
			r.Triggered = true
			s.activeAlarms[r.ID] = r
			cp := *r
			newlyDue = append(newlyDue, &cp)
		}
	}

	if len(newlyDue) > 0 {
		_ = s.saveLocked()
	}

	return newlyDue
}

func (s *Store) GetActiveAlarms() []*Reminder {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Reminder, 0, len(s.activeAlarms))
	for _, r := range s.activeAlarms {
		cp := *r
		list = append(list, &cp)
	}
	return list
}

type AppSettings struct {
	DarkMode bool `json:"dark_mode"`
}

func LoadSettings(dir string) AppSettings {
	filePath := filepath.Join(dir, "settings.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return AppSettings{DarkMode: false}
	}
	var s AppSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return AppSettings{DarkMode: false}
	}
	return s
}

func SaveSettings(dir string, s AppSettings) error {
	filePath := filepath.Join(dir, "settings.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}
