$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

$size = 1024
$bmp = New-Object System.Drawing.Bitmap $size, $size
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$g.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAliasGridFit
$g.Clear([System.Drawing.Color]::Transparent)

$bg = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::FromArgb(255, 28, 32, 40))
$path = New-Object System.Drawing.Drawing2D.GraphicsPath
$r = 180
$d = $r * 2
$x = 48
$y = 48
$w = 928
$h = 928
$path.AddArc($x, $y, $d, $d, 180, 90)
$path.AddArc(($x + $w - $d), $y, $d, $d, 270, 90)
$path.AddArc(($x + $w - $d), ($y + $h - $d), $d, $d, 0, 90)
$path.AddArc($x, ($y + $h - $d), $d, $d, 90, 90)
$path.CloseFigure()
$g.FillPath($bg, $path)

$fontStyle = [System.Drawing.FontStyle]::Bold
$font = New-Object System.Drawing.Font('Segoe UI', 280, $fontStyle, [System.Drawing.GraphicsUnit]::Pixel)
$fg = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::FromArgb(255, 243, 244, 246))
$sf = New-Object System.Drawing.StringFormat
$sf.Alignment = [System.Drawing.StringAlignment]::Center
$sf.LineAlignment = [System.Drawing.StringAlignment]::Center
$g.DrawString('AD', $font, $fg, (New-Object System.Drawing.RectangleF 0, 40, $size, $size), $sf)

$root = Split-Path -Parent $PSScriptRoot
$out = Join-Path $root 'shell\src-tauri\app-icon.png'
$bmp.Save($out, [System.Drawing.Imaging.ImageFormat]::Png)
$g.Dispose()
$bmp.Dispose()
Write-Host "Wrote $out"
