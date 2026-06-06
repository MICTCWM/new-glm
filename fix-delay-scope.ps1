# PowerShell script to fix delay variable scope errors in Go files

$files = @(
    "relay\compatible_handler.go",
    "relay\claude_handler.go",
    "relay\gemini_handler.go",
    "relay\chat_completions_via_responses.go"
)

$pattern = 'if delay := common\.RetryDelays\[0\]; len\(common\.RetryDelays\) > 0 && attempt < len\(common\.RetryDelays\) \{\s+delay = common\.RetryDelays\[attempt\]\s+\}'
$replacement = 'var delay time.Duration
			if len(common.RetryDelays) > 0 && attempt < len(common.RetryDelays) {
				delay = common.RetryDelays[attempt]
			}'

foreach ($file in $files) {
    Write-Host "Processing $file"
    
    $content = Get-Content $file -Raw -Encoding UTF8
    
    # Fix all instances of the delay variable scope error
    $content = $content -replace $pattern, $replacement
    
    Set-Content $file -Value $content -Encoding UTF8 -NoNewline
    
    Write-Host "Fixed $file"
}

Write-Host "All files have been fixed!"