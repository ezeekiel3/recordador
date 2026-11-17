package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

var (
	soundKeys         = []string{"campana", "digital", "melodia", "ding"}
	soundDisplayNames = []string{"🔔 Campana", "⏱️ Despertador Digital", "🎵 Melodía Armónica", "✨ Ding Rápido"}

	dwmapi                    = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

// setWindowDarkMode activa o desactiva la barra de título oscura inmersiva de Windows 10/11
func setWindowDarkMode(hwnd win.HWND, enable bool) {
	var val int32
	if enable {
		val = 1
	}
	// Atributo 20 para Windows 10 20H1+ y Windows 11
	r, _, _ := procDwmSetWindowAttribute.Call(
		uintptr(hwnd),
		20,
		uintptr(unsafe.Pointer(&val)),
		unsafe.Sizeof(val),
	)
	if r != 0 {
		// Fallback atributo 19 para compilaciones anteriores de Windows 10
		procDwmSetWindowAttribute.Call(
			uintptr(hwnd),
			19,
			uintptr(unsafe.Pointer(&val)),
			unsafe.Sizeof(val),
		)
	}
}

type ReminderModel struct {
	walk.TableModelBase
	allReminders      []*Reminder
	filteredReminders []*Reminder
	filterType        string // "all", "pending", "completed"
	searchQuery       string
	isDark            bool
}

func (m *ReminderModel) RowCount() int {
	return len(m.filteredReminders)
}

func (m *ReminderModel) Value(row, col int) interface{} {
	if row < 0 || row >= len(m.filteredReminders) {
		return ""
	}
	r := m.filteredReminders[row]
	switch col {
	case 0:
		if r.Completed {
			return "✅ Completado"
		}
		if time.Now().After(r.DueTime) {
			return "⚠️ ¡Vencido!"
		}
		return "⏳ Pendiente"
	case 1:
		return r.Title
	case 2:
		return r.DueTime.Format("02/01/2006 15:04")
	case 3:
		switch r.SoundType {
		case "digital":
			return "⏱️ Despertador"
		case "melodia":
			return "🎵 Melodía"
		case "ding":
			return "✨ Ding"
		default:
			return "🔔 Campana"
		}
	case 4:
		clean := strings.ReplaceAll(r.Text, "\r\n", " ")
		clean = strings.ReplaceAll(clean, "\n", " ")
		return clean
	}
	return ""
}

// StyleCell aplica los colores del tema oscuro o claro en cada fila de la tabla
func (m *ReminderModel) StyleCell(style *walk.CellStyle) {
	if m.isDark {
		if style.Row()%2 == 0 {
			style.BackgroundColor = walk.RGB(30, 41, 59) // Slate 800
		} else {
			style.BackgroundColor = walk.RGB(15, 23, 42) // Slate 900
		}
		style.TextColor = walk.RGB(248, 250, 252) // Blanco suave
	} else {
		if style.Row()%2 == 0 {
			style.BackgroundColor = walk.RGB(255, 255, 255)
		} else {
			style.BackgroundColor = walk.RGB(248, 250, 252)
		}
		style.TextColor = walk.RGB(15, 23, 42)
	}
}

func (m *ReminderModel) ApplyFilter(all []*Reminder) {
	m.allReminders = all
	var filtered []*Reminder
	q := strings.ToLower(strings.TrimSpace(m.searchQuery))

	for _, r := range all {
		if m.filterType == "pending" && r.Completed {
			continue
		}
		if m.filterType == "completed" && !r.Completed {
			continue
		}

		if q != "" {
			matchTitle := strings.Contains(strings.ToLower(r.Title), q)
			matchText := strings.Contains(strings.ToLower(r.Text), q)
			if !matchTitle && !matchText {
				continue
			}
		}

		filtered = append(filtered, r)
	}

	m.filteredReminders = filtered
	m.PublishRowsReset()
}

func (m *ReminderModel) GetItem(row int) *Reminder {
	if row >= 0 && row < len(m.filteredReminders) {
		return m.filteredReminders[row]
	}
	return nil
}

var (
	alarmMu        sync.Mutex
	activeAlarmDlg *walk.Dialog
)

func main() {
	// Determinar ruta de almacenamiento junto al ejecutable
	exePath, err := os.Executable()
	var baseDir string
	if err == nil {
		baseDir = filepath.Dir(exePath)
	} else {
		baseDir = "."
	}
	dbPath := filepath.Join(baseDir, "recordatorios.json")
	iconPath := filepath.Join(baseDir, "icon.ico")

	settings := LoadSettings(baseDir)
	isDark := settings.DarkMode

	store, err := NewStore(dbPath)
	if err != nil {
		log.Fatalf("Error inicializando almacenamiento: %v", err)
	}

	model := &ReminderModel{
		filterType: "all",
		isDark:     isDark,
	}
	model.ApplyFilter(store.GetAll())

	var mw *walk.MainWindow
	var gbForm *walk.GroupBox
	var gbList *walk.GroupBox
	var inTitle *walk.LineEdit
	var inText *walk.TextEdit
	var inDate *walk.DateEdit
	var inSound *walk.ComboBox
	var tvReminders *walk.TableView
	var inSearch *walk.LineEdit
	var inFilter *walk.ComboBox
	var lblStats *walk.Label
	var btnTheme *walk.PushButton

	// Pinceles para temas
	brushDarkBg, _ := walk.NewSolidColorBrush(walk.RGB(24, 24, 27))    // #18181B
	brushDarkCard, _ := walk.NewSolidColorBrush(walk.RGB(39, 39, 42))  // #27272A
	brushLightBg, _ := walk.NewSolidColorBrush(walk.RGB(248, 250, 252)) // #F8FAFC
	brushLightCard, _ := walk.NewSolidColorBrush(walk.RGB(255, 255, 255))

	applyTheme := func(dark bool) {
		isDark = dark
		model.isDark = dark
		model.PublishRowsReset()

		if mw != nil {
			setWindowDarkMode(mw.Handle(), dark)
			if dark {
				mw.SetBackground(brushDarkBg)
				if gbForm != nil {
					gbForm.SetBackground(brushDarkCard)
				}
				if gbList != nil {
					gbList.SetBackground(brushDarkCard)
				}
				if btnTheme != nil {
					btnTheme.SetText("☀️ Modo Claro")
				}
			} else {
				mw.SetBackground(brushLightBg)
				if gbForm != nil {
					gbForm.SetBackground(brushLightCard)
				}
				if gbList != nil {
					gbList.SetBackground(brushLightCard)
				}
				if btnTheme != nil {
					btnTheme.SetText("🌙 Modo Oscuro")
				}
			}
			mw.Invalidate()
		}

		_ = SaveSettings(baseDir, AppSettings{DarkMode: dark})
	}

	updateStats := func() {
		all := store.GetAll()
		total := len(all)
		completed := 0
		for _, r := range all {
			if r.Completed {
				completed++
			}
		}
		pending := total - completed
		if lblStats != nil {
			lblStats.SetText(fmt.Sprintf("Total: %d  |  Pendientes: %d  |  Completados: %d", total, pending, completed))
		}
	}

	refreshAll := func() {
		model.ApplyFilter(store.GetAll())
		updateStats()
	}

	defaultTime := time.Now().Add(15 * time.Minute)
	filterOptions := []string{"Todos los recordatorios", "Solo Pendientes", "Solo Completados"}

	themeBtnText := "🌙 Modo Oscuro"
	if isDark {
		themeBtnText = "☀️ Modo Claro"
	}

	err = MainWindow{
		AssignTo: &mw,
		Title:    "⏰ Recordador & Notas - Golang",
		Icon:     iconPath,
		MinSize:  Size{Width: 840, Height: 640},
		Size:     Size{Width: 890, Height: 690},
		Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 10},
		Children: []Widget{
			// SECCIÓN SUPERIOR: FORMULARIO PARA AGREGAR
			GroupBox{
				AssignTo: &gbForm,
				Title:    "➕ Nuevo Recordatorio / Nota",
				Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 8},
				Children: []Widget{
					Composite{
						Layout: HBox{MarginsZero: true, Spacing: 8},
						Children: []Widget{
							Label{Text: "Título / Asunto:", MinSize: Size{Width: 100, Height: 0}},
							LineEdit{
								AssignTo: &inTitle,
							},
							PushButton{
								AssignTo: &btnTheme,
								Text:     themeBtnText,
								OnClicked: func() {
									applyTheme(!isDark)
								},
							},
						},
					},
					Composite{
						Layout: HBox{MarginsZero: true, Spacing: 8},
						Children: []Widget{
							Label{Text: "Detalle / Notas:", MinSize: Size{Width: 100, Height: 0}},
							TextEdit{
								AssignTo: &inText,
								MinSize:  Size{Width: 0, Height: 55},
							},
						},
					},
					// Fila de Fecha, Hora y Atajos rápidos
					Composite{
						Layout: HBox{MarginsZero: true, Spacing: 8},
						Children: []Widget{
							Label{Text: "Fecha y Alarma:", MinSize: Size{Width: 100, Height: 0}},
							DateEdit{
								AssignTo: &inDate,
								Format:   "dd/MM/yyyy HH:mm",
								Date:     defaultTime,
							},
							Label{Text: "Atajos rápidos:"},
							PushButton{
								Text: "+5 min",
								OnClicked: func() {
									inDate.SetDate(time.Now().Add(5 * time.Minute))
								},
							},
							PushButton{
								Text: "+15 min",
								OnClicked: func() {
									inDate.SetDate(time.Now().Add(15 * time.Minute))
								},
							},
							PushButton{
								Text: "+30 min",
								OnClicked: func() {
									inDate.SetDate(time.Now().Add(30 * time.Minute))
								},
							},
							PushButton{
								Text: "+1 hora",
								OnClicked: func() {
									inDate.SetDate(time.Now().Add(1 * time.Hour))
								},
							},
							PushButton{
								Text: "Mañana 9am",
								OnClicked: func() {
									now := time.Now()
									tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 9, 0, 0, 0, now.Location())
									inDate.SetDate(tomorrow)
								},
							},
							HSpacer{},
						},
					},
					// Fila de Selección de Sonido y Botón Guardar
					Composite{
						Layout: HBox{MarginsZero: true, Spacing: 8},
						Children: []Widget{
							Label{Text: "Sonido Alarma:", MinSize: Size{Width: 100, Height: 0}},
							ComboBox{
								AssignTo:     &inSound,
								Model:        soundDisplayNames,
								CurrentIndex: 0,
							},
							PushButton{
								Text: "🔊 Probar",
								OnClicked: func() {
									idx := inSound.CurrentIndex()
									if idx >= 0 && idx < len(soundKeys) {
										PlaySystemSound(soundKeys[idx])
									}
								},
							},
							HSpacer{},
							PushButton{
								Text: "💾 Guardar Recordatorio",
								Font: Font{Bold: true},
								OnClicked: func() {
									title := strings.TrimSpace(inTitle.Text())
									if title == "" {
										walk.MsgBox(mw, "Campo Requerido", "Por favor ingresa un título para el recordatorio.", walk.MsgBoxIconWarning)
										return
									}
									soundIdx := inSound.CurrentIndex()
									soundKey := "campana"
									if soundIdx >= 0 && soundIdx < len(soundKeys) {
										soundKey = soundKeys[soundIdx]
									}
									dueTime := inDate.Date()

									_, err := store.Create(title, inText.Text(), dueTime, soundKey)
									if err != nil {
										walk.MsgBox(mw, "Error al Guardar", err.Error(), walk.MsgBoxIconError)
										return
									}

									inTitle.SetText("")
									inText.SetText("")
									inDate.SetDate(time.Now().Add(15 * time.Minute))
									refreshAll()
									inTitle.SetFocus()
								},
							},
							PushButton{
								Text: "Limpiar",
								OnClicked: func() {
									inTitle.SetText("")
									inText.SetText("")
									inDate.SetDate(time.Now().Add(15 * time.Minute))
								},
							},
						},
					},
				},
			},

			// SECCIÓN INFERIOR: LISTA DE RECORDATORIOS
			GroupBox{
				AssignTo: &gbList,
				Title:    "📋 Lista de Recordatorios y Notas",
				Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 8},
				Children: []Widget{
					// Fila de Búsqueda y Filtro
					Composite{
						Layout: HBox{MarginsZero: true, Spacing: 8},
						Children: []Widget{
							Label{Text: "🔍 Buscar:"},
							LineEdit{
								AssignTo: &inSearch,
								OnTextChanged: func() {
									model.searchQuery = inSearch.Text()
									model.ApplyFilter(store.GetAll())
								},
							},
							Label{Text: "Filtrar por:"},
							ComboBox{
								AssignTo:     &inFilter,
								Model:        filterOptions,
								CurrentIndex: 0,
								OnCurrentIndexChanged: func() {
									switch inFilter.CurrentIndex() {
									case 1:
										model.filterType = "pending"
									case 2:
										model.filterType = "completed"
									default:
										model.filterType = "all"
									}
									model.ApplyFilter(store.GetAll())
								},
							},
							HSpacer{},
							Label{
								AssignTo: &lblStats,
								Text:     "Cargando estadísticas...",
							},
						},
					},

					// Tabla principal
					TableView{
						AssignTo:         &tvReminders,
						Model:            model,
						AlternatingRowBG: true,
						Columns: []TableViewColumn{
							{Title: "Estado", Width: 105},
							{Title: "Título", Width: 210},
							{Title: "Fecha y Hora", Width: 130},
							{Title: "Sonido", Width: 110},
							{Title: "Notas / Detalle", Width: 280},
						},
						OnItemActivated: func() {
							idx := tvReminders.CurrentIndex()
							if item := model.GetItem(idx); item != nil {
								showEditDialog(mw, store, refreshAll, item, isDark, iconPath)
							}
						},
					},

					// Botones de acción sobre el elemento seleccionado
					Composite{
						Layout: HBox{MarginsZero: true, Spacing: 8},
						Children: []Widget{
							PushButton{
								Text: "✅ Completar / Reactivar",
								OnClicked: func() {
									idx := tvReminders.CurrentIndex()
									item := model.GetItem(idx)
									if item == nil {
										walk.MsgBox(mw, "Selección", "Por favor selecciona un recordatorio de la lista.", walk.MsgBoxIconInformation)
										return
									}
									_, _ = store.ToggleComplete(item.ID)
									refreshAll()
								},
							},
							PushButton{
								Text: "✏️ Editar",
								OnClicked: func() {
									idx := tvReminders.CurrentIndex()
									item := model.GetItem(idx)
									if item == nil {
										walk.MsgBox(mw, "Selección", "Por favor selecciona un recordatorio de la lista.", walk.MsgBoxIconInformation)
										return
									}
									showEditDialog(mw, store, refreshAll, item, isDark, iconPath)
								},
							},
							PushButton{
								Text: "🗑️ Eliminar",
								OnClicked: func() {
									idx := tvReminders.CurrentIndex()
									item := model.GetItem(idx)
									if item == nil {
										walk.MsgBox(mw, "Selección", "Por favor selecciona un recordatorio de la lista.", walk.MsgBoxIconInformation)
										return
									}
									msg := fmt.Sprintf("¿Estás seguro de que deseas eliminar '%s'?", item.Title)
									if walk.MsgBox(mw, "Confirmar eliminación", msg, walk.MsgBoxIconQuestion|walk.MsgBoxYesNo) == walk.DlgCmdYes {
										_ = store.Delete(item.ID)
										refreshAll()
									}
								},
							},
							PushButton{
								Text: "⏰ Posponer (+5 min)",
								OnClicked: func() {
									idx := tvReminders.CurrentIndex()
									item := model.GetItem(idx)
									if item == nil {
										walk.MsgBox(mw, "Selección", "Por favor selecciona un recordatorio de la lista.", walk.MsgBoxIconInformation)
										return
									}
									_, _ = store.Snooze(item.ID, 5)
									refreshAll()
								},
							},
							PushButton{
								Text: "🔊 Escuchar Alarma",
								OnClicked: func() {
									idx := tvReminders.CurrentIndex()
									item := model.GetItem(idx)
									if item == nil {
										PlaySystemSound("campana")
										return
									}
									PlaySystemSound(item.SoundType)
								},
							},
						},
					},
				},
			},
		},
	}.Create()

	if err != nil {
		log.Fatalf("Error creando la ventana principal: %v", err)
	}

	// Aplicar tema inicial
	applyTheme(isDark)
	updateStats()

	// Chequeador de alarmas en segundo plano cada 1 segundo
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			dueItems := store.CheckDue()
			for _, item := range dueItems {
				itemCopy := *item
				mw.Synchronize(func() {
					refreshAll()
					showAlarmDialog(mw, store, refreshAll, &itemCopy, isDark, iconPath)
				})
			}
		}
	}()

	// Refrescar periódicamente la lista cada 15 segundos
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			mw.Synchronize(func() {
				model.ApplyFilter(store.GetAll())
			})
		}
	}()

	mw.Run()
}

// showEditDialog abre una ventana modal nativa para modificar un recordatorio existente
func showEditDialog(owner walk.Form, store *Store, onUpdated func(), item *Reminder, isDark bool, iconPath string) {
	var dlg *walk.Dialog
	var editTitle *walk.LineEdit
	var editText *walk.TextEdit
	var editDate *walk.DateEdit
	var editSound *walk.ComboBox

	selectedSoundIdx := 0
	for i, k := range soundKeys {
		if k == item.SoundType {
			selectedSoundIdx = i
			break
		}
	}

	err := Dialog{
		AssignTo: &dlg,
		Title:    "✏️ Editar Recordatorio",
		Icon:     iconPath,
		MinSize:  Size{Width: 480, Height: 350},
		Size:     Size{Width: 500, Height: 360},
		Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 8},
		Children: []Widget{
			Label{Text: "Título / Asunto:"},
			LineEdit{
				AssignTo: &editTitle,
				Text:     item.Title,
			},
			Label{Text: "Detalle / Notas adicionales:"},
			TextEdit{
				AssignTo: &editText,
				Text:     item.Text,
				MinSize:  Size{Width: 0, Height: 60},
			},
			Label{Text: "Fecha y Hora de la Alarma:"},
			DateEdit{
				AssignTo: &editDate,
				Format:   "dd/MM/yyyy HH:mm",
				Date:     item.DueTime,
			},
			Label{Text: "Sonido de la Alarma:"},
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 8},
				Children: []Widget{
					ComboBox{
						AssignTo:     &editSound,
						Model:        soundDisplayNames,
						CurrentIndex: selectedSoundIdx,
					},
					PushButton{
						Text: "🔊 Probar",
						OnClicked: func() {
							idx := editSound.CurrentIndex()
							if idx >= 0 && idx < len(soundKeys) {
								PlaySystemSound(soundKeys[idx])
							}
						},
					},
				},
			},
			VSpacer{},
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 8},
				Children: []Widget{
					HSpacer{},
					PushButton{
						Text: "💾 Guardar Cambios",
						Font: Font{Bold: true},
						OnClicked: func() {
							title := strings.TrimSpace(editTitle.Text())
							if title == "" {
								walk.MsgBox(dlg, "Campo requerido", "El título no puede estar vacío.", walk.MsgBoxIconWarning)
								return
							}
							soundIdx := editSound.CurrentIndex()
							soundKey := "campana"
							if soundIdx >= 0 && soundIdx < len(soundKeys) {
								soundKey = soundKeys[soundIdx]
							}

							_, err := store.Update(item.ID, title, editText.Text(), editDate.Date(), soundKey)
							if err != nil {
								walk.MsgBox(dlg, "Error", err.Error(), walk.MsgBoxIconError)
								return
							}

							if onUpdated != nil {
								onUpdated()
							}
							dlg.Accept()
						},
					},
					PushButton{
						Text: "Cancelar",
						OnClicked: func() {
							dlg.Cancel()
						},
					},
				},
			},
		},
	}.Create(owner)

	if err == nil {
		setWindowDarkMode(dlg.Handle(), isDark)
		dlg.Run()
	}
}

// showAlarmDialog muestra la ventana de alarma con reproducción continua de sonido
func showAlarmDialog(owner *walk.MainWindow, store *Store, onUpdated func(), item *Reminder, isDark bool, iconPath string) {
	alarmMu.Lock()
	if activeAlarmDlg != nil {
		alarmMu.Unlock()
		return
	}
	alarmMu.Unlock()

	stopSound := make(chan struct{})

	// Bucle de sonido mientras la ventana de alarma esté en pantalla
	go func() {
		PlaySystemSound(item.SoundType)
		ticker := time.NewTicker(2600 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				PlaySystemSound(item.SoundType)
			case <-stopSound:
				return
			}
		}
	}()

	var dlg *walk.Dialog

	err := Dialog{
		AssignTo: &dlg,
		Title:    "🔔 ¡RECORDATORIO ACTIVADO!",
		Icon:     iconPath,
		MinSize:  Size{Width: 460, Height: 260},
		Size:     Size{Width: 480, Height: 280},
		Layout:   VBox{Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}, Spacing: 10},
		Children: []Widget{
			Label{
				Text: "⏰ ¡HORA DE TU RECORDATORIO!",
				Font: Font{PointSize: 12, Bold: true},
			},
			Label{
				Text: fmt.Sprintf("Asunto: %s", item.Title),
				Font: Font{PointSize: 11, Bold: true},
			},
			Label{
				Text: fmt.Sprintf("Programado para: %s", item.DueTime.Format("02/01/2006 15:04")),
			},
			TextEdit{
				Text:     item.Text,
				ReadOnly: true,
				MinSize:  Size{Width: 0, Height: 65},
			},
			VSpacer{},
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 8},
				Children: []Widget{
					PushButton{
						Text: "✅ Entendido / Silenciar",
						Font: Font{Bold: true},
						OnClicked: func() {
							_, _ = store.DismissAlarm(item.ID)
							if onUpdated != nil {
								onUpdated()
							}
							dlg.Accept()
						},
					},
					PushButton{
						Text: "⏰ Posponer (+5 min)",
						OnClicked: func() {
							_, _ = store.Snooze(item.ID, 5)
							if onUpdated != nil {
								onUpdated()
							}
							dlg.Accept()
						},
					},
					PushButton{
						Text: "✔️ Marcar Completado",
						OnClicked: func() {
							_, _ = store.ToggleComplete(item.ID)
							if onUpdated != nil {
								onUpdated()
							}
							dlg.Accept()
						},
					},
				},
			},
		},
	}.Create(owner)

	if err != nil {
		close(stopSound)
		return
	}

	setWindowDarkMode(dlg.Handle(), isDark)

	alarmMu.Lock()
	activeAlarmDlg = dlg
	alarmMu.Unlock()

	dlg.SetVisible(true)
	dlg.BringToTop()

	dlg.Run()

	close(stopSound)
	alarmMu.Lock()
	activeAlarmDlg = nil
	alarmMu.Unlock()
}
