package main

import (
	"fmt"
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
	r, _, _ := procDwmSetWindowAttribute.Call(
		uintptr(hwnd),
		20, // DWMWA_USE_IMMERSIVE_DARK_MODE para Windows 10 20H1+ y Windows 11
		uintptr(unsafe.Pointer(&val)),
		unsafe.Sizeof(val),
	)
	if r != 0 {
		procDwmSetWindowAttribute.Call(
			uintptr(hwnd),
			19, // Fallback para Windows 10 1809-1909
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

// StyleCell aplica los colores adecuados en las filas de la tabla
func (m *ReminderModel) StyleCell(style *walk.CellStyle) {
	if m.isDark {
		if style.Row()%2 == 0 {
			style.BackgroundColor = walk.RGB(28, 33, 48) // Azul grisáceo oscuro
		} else {
			style.BackgroundColor = walk.RGB(20, 24, 35) // Azul noche profundo
		}
		style.TextColor = walk.RGB(245, 247, 250) // Texto blanco suave
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

// trySetWindowIcon intenta establecer el icono en la barra de título de la ventana
func trySetWindowIcon(hwnd win.HWND, iconPath string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	procLoadImageW := user32.NewProc("LoadImageW")
	procSendMessageW := user32.NewProc("SendMessageW")

	ptr, err := syscall.UTF16PtrFromString(iconPath)
	if err != nil {
		return
	}
	hIcon, _, _ := procLoadImageW.Call(
		0,
		uintptr(unsafe.Pointer(ptr)),
		1, // IMAGE_ICON
		0, 0,
		0x00000010|0x00000040, // LR_LOADFROMFILE | LR_DEFAULTSIZE
	)
	if hIcon != 0 {
		procSendMessageW.Call(uintptr(hwnd), 0x0080, 0, hIcon) // WM_SETICON SMALL
		procSendMessageW.Call(uintptr(hwnd), 0x0080, 1, hIcon) // WM_SETICON BIG
	}
}

var (
	alarmMu        sync.Mutex
	activeAlarmDlg *walk.Dialog
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("Error crítico en la aplicación:\n%v", r)
			user32 := syscall.NewLazyDLL("user32.dll")
			pMsg, _ := syscall.UTF16PtrFromString(msg)
			pTitle, _ := syscall.UTF16PtrFromString("Error - Recordador")
			user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(pMsg)), uintptr(unsafe.Pointer(pTitle)), 0x10)
		}
	}()

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
	currentVol := settings.Volume
	if currentVol <= 0 {
		currentVol = 80
	}
	SetGlobalVolume(currentVol)

	store, err := NewStore(dbPath)
	if err != nil {
		msg := fmt.Sprintf("Error inicializando almacenamiento:\n%v", err)
		user32 := syscall.NewLazyDLL("user32.dll")
		pMsg, _ := syscall.UTF16PtrFromString(msg)
		pTitle, _ := syscall.UTF16PtrFromString("Error - Recordador")
		user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(pMsg)), uintptr(unsafe.Pointer(pTitle)), 0x10)
		return
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
	var sliderVol *walk.Slider
	var lblVol *walk.Label
	var tvReminders *walk.TableView
	var inSearch *walk.LineEdit
	var inFilter *walk.ComboBox
	var lblStats *walk.Label
	var btnTheme *walk.PushButton

	// Etiquetas para actualizar su color en modo oscuro
	var lblTitle, lblText, lblDate, lblShortcuts, lblSound, lblVolTitle, lblSearch, lblFilterTitle *walk.Label

	// Pinceles de tema
	brushDarkBg, _ := walk.NewSolidColorBrush(walk.RGB(18, 20, 29))     // Fondo general oscuro
	brushDarkCard, _ := walk.NewSolidColorBrush(walk.RGB(28, 33, 48))   // Tarjetas oscuras
	brushDarkInput, _ := walk.NewSolidColorBrush(walk.RGB(38, 44, 64))  // Cajas de texto oscuras
	brushDarkTable, _ := walk.NewSolidColorBrush(walk.RGB(20, 24, 35))  // Tabla oscura

	brushLightBg, _ := walk.NewSolidColorBrush(walk.RGB(245, 247, 250)) // Fondo claro
	brushLightCard, _ := walk.NewSolidColorBrush(walk.RGB(255, 255, 255))
	brushLightInput, _ := walk.NewSolidColorBrush(walk.RGB(255, 255, 255))
	brushLightTable, _ := walk.NewSolidColorBrush(walk.RGB(255, 255, 255))

	applyTheme := func(dark bool) {
		isDark = dark
		model.isDark = dark
		model.PublishRowsReset()

		allLabels := []*walk.Label{
			lblTitle, lblText, lblDate, lblShortcuts, lblSound, lblVolTitle, lblVol,
			lblSearch, lblFilterTitle, lblStats,
		}

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
				if tvReminders != nil {
					tvReminders.SetBackground(brushDarkTable)
				}
				if inTitle != nil {
					inTitle.SetBackground(brushDarkInput)
					inTitle.SetTextColor(walk.RGB(255, 255, 255))
				}
				if inText != nil {
					inText.SetBackground(brushDarkInput)
					inText.SetTextColor(walk.RGB(255, 255, 255))
				}
				if inSearch != nil {
					inSearch.SetBackground(brushDarkInput)
					inSearch.SetTextColor(walk.RGB(255, 255, 255))
				}
				for _, lbl := range allLabels {
					if lbl != nil {
						lbl.SetTextColor(walk.RGB(230, 235, 245))
					}
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
				if tvReminders != nil {
					tvReminders.SetBackground(brushLightTable)
				}
				if inTitle != nil {
					inTitle.SetBackground(brushLightInput)
					inTitle.SetTextColor(walk.RGB(15, 23, 42))
				}
				if inText != nil {
					inText.SetBackground(brushLightInput)
					inText.SetTextColor(walk.RGB(15, 23, 42))
				}
				if inSearch != nil {
					inSearch.SetBackground(brushLightInput)
					inSearch.SetTextColor(walk.RGB(15, 23, 42))
				}
				for _, lbl := range allLabels {
					if lbl != nil {
						lbl.SetTextColor(walk.RGB(15, 23, 42))
					}
				}
				if btnTheme != nil {
					btnTheme.SetText("🌙 Modo Oscuro")
				}
			}
			mw.Invalidate()
		}

		_ = SaveSettings(baseDir, AppSettings{DarkMode: dark, Volume: GetGlobalVolume()})
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
		MinSize:  Size{Width: 860, Height: 660},
		Size:     Size{Width: 920, Height: 710},
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
							Label{AssignTo: &lblTitle, Text: "Título / Asunto:", MinSize: Size{Width: 100, Height: 0}},
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
							Label{AssignTo: &lblText, Text: "Detalle / Notas:", MinSize: Size{Width: 100, Height: 0}},
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
							Label{AssignTo: &lblDate, Text: "Fecha y Alarma:", MinSize: Size{Width: 100, Height: 0}},
							DateEdit{
								AssignTo: &inDate,
								Format:   "dd/MM/yyyy HH:mm",
								Date:     defaultTime,
							},
							Label{AssignTo: &lblShortcuts, Text: "Atajos:"},
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
					// Fila de Selección de Sonido, Barra de Volumen y Botón Guardar
					Composite{
						Layout: HBox{MarginsZero: true, Spacing: 8},
						Children: []Widget{
							Label{AssignTo: &lblSound, Text: "Sonido Alarma:", MinSize: Size{Width: 100, Height: 0}},
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
							Label{AssignTo: &lblVolTitle, Text: "Volumen:"},
							Slider{
								AssignTo: &sliderVol,
								MinValue: 0,
								MaxValue: 100,
								Value:    currentVol,
								Tracking: true,
								MinSize:  Size{Width: 100, Height: 0},
								OnValueChanged: func() {
									val := sliderVol.Value()
									SetGlobalVolume(val)
									if lblVol != nil {
										lblVol.SetText(fmt.Sprintf("%d%%", val))
									}
									_ = SaveSettings(baseDir, AppSettings{DarkMode: isDark, Volume: val})
								},
							},
							Label{
								AssignTo: &lblVol,
								Text:     fmt.Sprintf("%d%%", currentVol),
								MinSize:  Size{Width: 38, Height: 0},
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
							Label{AssignTo: &lblSearch, Text: "🔍 Buscar:"},
							LineEdit{
								AssignTo: &inSearch,
								OnTextChanged: func() {
									model.searchQuery = inSearch.Text()
									model.ApplyFilter(store.GetAll())
								},
							},
							Label{AssignTo: &lblFilterTitle, Text: "Filtrar por:"},
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
								showEditDialog(mw, store, refreshAll, item, isDark)
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
									showEditDialog(mw, store, refreshAll, item, isDark)
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
		msg := fmt.Sprintf("Error al crear la ventana principal:\n%v", err)
		user32 := syscall.NewLazyDLL("user32.dll")
		pMsg, _ := syscall.UTF16PtrFromString(msg)
		pTitle, _ := syscall.UTF16PtrFromString("Error - Recordador")
		user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(pMsg)), uintptr(unsafe.Pointer(pTitle)), 0x10)
		return
	}

	// Aplicar tema inicial y establecer icono
	applyTheme(isDark)
	trySetWindowIcon(mw.Handle(), iconPath)
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
					showAlarmDialog(mw, store, refreshAll, &itemCopy, isDark)
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
func showEditDialog(owner walk.Form, store *Store, onUpdated func(), item *Reminder, isDark bool) {
	var dlg *walk.Dialog
	var editTitle *walk.LineEdit
	var editText *walk.TextEdit
	var editDate *walk.DateEdit
	var editSound *walk.ComboBox
	var lblETitle, lblEText, lblEDate, lblESound *walk.Label

	brushDarkCard, _ := walk.NewSolidColorBrush(walk.RGB(28, 33, 48))
	brushDarkInput, _ := walk.NewSolidColorBrush(walk.RGB(38, 44, 64))

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
		MinSize:  Size{Width: 480, Height: 350},
		Size:     Size{Width: 500, Height: 360},
		Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 8},
		Children: []Widget{
			Label{AssignTo: &lblETitle, Text: "Título / Asunto:"},
			LineEdit{
				AssignTo: &editTitle,
				Text:     item.Title,
			},
			Label{AssignTo: &lblEText, Text: "Detalle / Notas adicionales:"},
			TextEdit{
				AssignTo: &editText,
				Text:     item.Text,
				MinSize:  Size{Width: 0, Height: 60},
			},
			Label{AssignTo: &lblEDate, Text: "Fecha y Hora de la Alarma:"},
			DateEdit{
				AssignTo: &editDate,
				Format:   "dd/MM/yyyy HH:mm",
				Date:     item.DueTime,
			},
			Label{AssignTo: &lblESound, Text: "Sonido de la Alarma:"},
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
		if isDark {
			setWindowDarkMode(dlg.Handle(), true)
			dlg.SetBackground(brushDarkCard)
			editTitle.SetBackground(brushDarkInput)
			editTitle.SetTextColor(walk.RGB(255, 255, 255))
			editText.SetBackground(brushDarkInput)
			editText.SetTextColor(walk.RGB(255, 255, 255))
			for _, l := range []*walk.Label{lblETitle, lblEText, lblEDate, lblESound} {
				if l != nil {
					l.SetTextColor(walk.RGB(230, 235, 245))
				}
			}
		}
		dlg.Run()
	}
}

// showAlarmDialog muestra la ventana de alarma con reproducción continua de sonido
func showAlarmDialog(owner *walk.MainWindow, store *Store, onUpdated func(), item *Reminder, isDark bool) {
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
	var lblHeader, lblSubject, lblTime *walk.Label
	var teNotes *walk.TextEdit

	brushDarkCard, _ := walk.NewSolidColorBrush(walk.RGB(28, 33, 48))
	brushDarkInput, _ := walk.NewSolidColorBrush(walk.RGB(38, 44, 64))

	err := Dialog{
		AssignTo: &dlg,
		Title:    "🔔 ¡RECORDATORIO ACTIVADO!",
		MinSize:  Size{Width: 460, Height: 260},
		Size:     Size{Width: 480, Height: 280},
		Layout:   VBox{Margins: Margins{Left: 14, Top: 14, Right: 14, Bottom: 14}, Spacing: 10},
		Children: []Widget{
			Label{
				AssignTo: &lblHeader,
				Text:     "⏰ ¡HORA DE TU RECORDATORIO!",
				Font:     Font{PointSize: 12, Bold: true},
			},
			Label{
				AssignTo: &lblSubject,
				Text:     fmt.Sprintf("Asunto: %s", item.Title),
				Font:     Font{PointSize: 11, Bold: true},
			},
			Label{
				AssignTo: &lblTime,
				Text:     fmt.Sprintf("Programado para: %s", item.DueTime.Format("02/01/2006 15:04")),
			},
			TextEdit{
				AssignTo: &teNotes,
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

	if isDark {
		setWindowDarkMode(dlg.Handle(), true)
		dlg.SetBackground(brushDarkCard)
		teNotes.SetBackground(brushDarkInput)
		teNotes.SetTextColor(walk.RGB(255, 255, 255))
		for _, l := range []*walk.Label{lblHeader, lblSubject, lblTime} {
			if l != nil {
				l.SetTextColor(walk.RGB(230, 235, 245))
			}
		}
	}

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
