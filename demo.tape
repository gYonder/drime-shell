Output demo.gif
Set Theme "Catppuccin Mocha"
Set FontFamily "Hack"
Set FontSize 16
Set Width 900
Set Height 500
Set Padding 20
Set BorderRadius 10
Set Margin 20
Set MarginFill "#1e1e2e"
Set WindowBar Colorful
Set CursorBlink true
Set TypingSpeed 100ms

# Launch drime-shell
Type "drime-shell"
Enter
Sleep 2.5s

# Find starred files, count, save to file
Type "find -S | wc -l > starred-count.txt"
Enter
Sleep 10.5s

# List to see the new file
Type "ls"
Enter
Sleep 2s

# Edit the file
Type "edit starred-count.txt"
Enter
Sleep 4.5s

# Exit editor with Ctrl+Q
Ctrl+q
Sleep 4s
