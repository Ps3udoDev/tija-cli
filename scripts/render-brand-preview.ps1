param([string]$Directory = "$env:TEMP\tija-brand-preview")
# Render the optional TestBrandPreview ANSI fixtures using a fixed cell grid.
# Usage: $env:TIJA_PREVIEW_DIR="$env:TEMP\tija-brand-preview"; go test ./internal/tui -run TestBrandPreview
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$font = [System.Drawing.Font]::new('Consolas', 15, [System.Drawing.FontStyle]::Regular, [System.Drawing.GraphicsUnit]::Pixel)
$format = [System.Drawing.StringFormat]::GenericTypographic
foreach ($file in Get-ChildItem -LiteralPath $Directory -Filter '*.ansi') {
    $lines = [IO.File]::ReadAllLines($file.FullName)
    $bitmap = [System.Drawing.Bitmap]::new(1240, (40 + $lines.Length * 20))
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $graphics.Clear([System.Drawing.ColorTranslator]::FromHtml('#081823'))
    $y = 20
    foreach ($line in $lines) {
        $foreground = '#FFFFFF'; $background = '#081823'; $x = 20
        foreach ($token in [regex]::Matches($line, '\x1b\[[0-9;]*m|[^\x1b]+')) {
            if ($token.Value.StartsWith([string][char]27)) {
                $codes = $token.Value.Substring(2, $token.Value.Length - 3)
                if ($codes -eq '' -or $codes -eq '0') { $foreground = '#FFFFFF'; $background = '#081823' }
                foreach ($match in [regex]::Matches($codes, '(38|48);2;(\d+);(\d+);(\d+)')) {
                    $color = '#{0:X2}{1:X2}{2:X2}' -f [int]$match.Groups[2].Value, [int]$match.Groups[3].Value, [int]$match.Groups[4].Value
                    if ($match.Groups[1].Value -eq '38') { $foreground = $color } else { $background = $color }
                }
            } else {
                foreach ($character in $token.Value.ToCharArray()) {
                    $front = [System.Drawing.SolidBrush]::new([System.Drawing.ColorTranslator]::FromHtml($foreground))
                    $back = [System.Drawing.SolidBrush]::new([System.Drawing.ColorTranslator]::FromHtml($background))
                    $graphics.FillRectangle($back, $x, $y, 10, 20)
                    switch ([int]$character) {
                        9608 { $graphics.FillRectangle($front, $x, $y, 10, 20) }
                        9600 { $graphics.FillRectangle($front, $x, $y, 10, 10) }
                        9604 { $graphics.FillRectangle($front, $x, ($y + 10), 10, 10) }
                        default { $graphics.DrawString([string]$character, $font, $front, [float]$x, [float]$y, $format) }
                    }
                    $front.Dispose(); $back.Dispose(); $x += 10
                }
            }
        }
        $y += 20
    }
    $bitmap.Save((Join-Path $Directory ($file.BaseName + '.png')), [System.Drawing.Imaging.ImageFormat]::Png)
    $graphics.Dispose(); $bitmap.Dispose()
}
$font.Dispose()
