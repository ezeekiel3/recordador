# ⏰ Recordador & Notas en Golang (Ventana Nativa de Windows)

Un programa completo de notas y recordatorios con **interfaz gráfica 100% nativa de Windows** (Win32) y alertas sonoras, desarrollado en **Go (Golang)**.

> **¡Sin navegadores web!** Funciona como un programa clásico y directo de escritorio de Windows (sin Edge, sin Chrome y sin servidores locales).

---

## ✨ Características Principales

1. **Interfaz Gráfica Nativa de Windows:**
   - Construida con controles nativos de Windows (`lxn/walk` y `Win32 API`).
   - Se abre como una ventana clásica independiente de Windows.
   - Tabla interactiva con columnas: *Estado*, *Título*, *Fecha y Hora*, *Sonido*, *Notas / Detalle*.
   - Buscador en tiempo real y selector de filtros (*Todos*, *Pendientes*, *Completados*).

2. **Anotar Título y Notas Detalladas:**
   - Campo para **Título / Asunto** y área multilinea para **Detalle / Notas**.
   - Selector de **Fecha y Hora** con formato `dd/MM/yyyy HH:mm`.
   - Botones rápidos: `+5 min`, `+15 min`, `+30 min`, `+1 hora`, `Mañana 9am`.

3. **Alertas Sonoras y Ventana Emergente:**
   - Al cumplirse la fecha y hora programada, reproduce una alarma sonora continua.
   - Sonidos configurables:
     - 🔔 **Campana**
     - ⏱️ **Despertador Digital**
     - 🎵 **Melodía Armónica**
     - ✨ **Ding Rápido**
   - Botón **"🔊 Probar"** para escuchar el sonido antes de guardar.
   - Al activarse la alarma, se muestra una ventana emergente en primer plano con opciones para:
     - **✅ Entendido / Silenciar**
     - **⏰ Posponer (+5 min)**
     - **✔️ Marcar como Completado**

4. **Gestión Completa (CRUD):**
   - **Agregar:** Crea nuevos recordatorios al instante.
   - **Editar:** Modifica título, texto, fecha/hora y sonido de cualquier recordatorio (botón *Editar* o doble clic en la tabla).
   - **Eliminar:** Borra notas que ya no necesites con confirmación.
   - **Completar / Reactivar:** Marca tareas finalizadas o vuelve a ponerlas como pendientes.

5. **Persistencia Automática:**
   - Guarda automáticamente todo en `recordatorios.json` junto al ejecutable.

---

## 🚀 Cómo Usarlo

### Opción 1: Desde tu Escritorio (La más fácil)
Ve a tu **Escritorio de Windows** y haz doble clic sobre el icono **`Recordador`**.

### Opción 2: Desde la carpeta del proyecto
Haz doble clic sobre:
- **`Recordador.exe`** o **`iniciar.bat`**

---

## 🔨 Cómo Compilar

Si deseas volver a compilar el ejecutable:
1. Haz doble clic en **`compilar.bat`**, o
2. Ejecuta en tu terminal:
```bash
go build -ldflags="-H windowsgui" -o Recordador.exe .
```

---

## 📂 Archivos del Proyecto

- `main.go`: Ventana principal nativa de Windows (Win32), formularios, tabla, diálogos y temporizadores de alarma.
- `storage.go`: Persistencia en `recordatorios.json` y operaciones seguras contra hilos (`sync.RWMutex`).
- `sound_windows.go`: Llamadas nativas a las APIs de sonido de Windows (`winmm.dll`, `kernel32.dll`, `user32.dll`).
- `Recordador.exe.manifest`: Manifiesto para estilos visuales modernos de Windows (Common Controls v6).
- `Recordador.exe`: El ejecutable final compilado.
- `compilar.bat`: Script de compilación rápida.
- `iniciar.bat`: Lanzador directo.
