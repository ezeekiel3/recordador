$ws = New-Object -ComObject WScript.Shell
$desktop = [System.Environment]::GetFolderPath('Desktop')
$target = "C:\Users\compupc\Documents\recordador\Recordador.exe"
$shortcutPath = Join-Path $desktop "Recordador.lnk"

$s = $ws.CreateShortcut($shortcutPath)
$s.TargetPath = $target
$s.WorkingDirectory = "C:\Users\compupc\Documents\recordador"
$s.Description = "Recordador y Notas con Alarma"
$s.IconLocation = "$target, 0"
$s.Save()

Write-Output "Acceso directo creado en: $shortcutPath"
