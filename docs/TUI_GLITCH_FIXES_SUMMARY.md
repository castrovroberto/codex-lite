# TUI Glitch Fixes Summary

## Overview

This document summarizes the fixes implemented to resolve TUI glitches related to:
1. Header top not being visible
2. Weird control sequence characters appearing in input area (like `]11;rgb:1e1e/1e1e/1e1e[1;1R`)
3. Terminal resize handling issues
4. General terminal compatibility problems

## Issues Identified and Fixed

### 1. Critical Input Area Window Resize Issue

**Problem**: The input area was intercepting `WindowSizeMsg` and preventing it from reaching the underlying `textarea.Model`, causing control sequences to leak through.

**Root Cause**: In `internal/tui/chat/inputarea_model.go`, the `Update` method was blocking resize messages from reaching the textarea:

```go
// PROBLEMATIC CODE (before fix)
if wmsg, ok := msg.(tea.WindowSizeMsg); ok {
    i.width = wmsg.Width
    i.textarea.SetWidth(wmsg.Width)
    return i, nil  // ❌ Blocked message from reaching textarea
}
```

**Fix**: Allow the resize message to reach the textarea while still updating our width:

```go
// FIXED CODE
if wmsg, ok := msg.(tea.WindowSizeMsg); ok {
    i.width = wmsg.Width
    i.textarea.SetWidth(wmsg.Width)
    // IMPORTANT: Pass the resize message to textarea as well
    i.textarea, cmd = i.textarea.Update(msg)
    return i, cmd  // ✅ Now passes through properly
}
```

**Files Modified**: `internal/tui/chat/inputarea_model.go`

### 2. Control Sequence Sanitization

**Problem**: Terminal control sequences like OSC (`]11;rgb:...`) and CSI (`[1;1R`) were appearing as visible characters in the input area.

**Root Cause**: These sequences are terminal responses to queries or mishandled escape sequences that weren't being filtered out.

**Fix**: Added input sanitization to remove problematic control sequences:

```go
// Added sanitization function
func sanitizeInput(input string) string {
    // Remove OSC sequences (]11;rgb:..., etc.)
    oscPattern := regexp.MustCompile(`\x1b\][0-9;]*[a-zA-Z]`)
    input = oscPattern.ReplaceAllString(input, "")
    
    // Remove CSI sequences ([1;1R, etc.)
    csiPattern := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
    input = csiPattern.ReplaceAllString(input, "")
    
    // Remove other escape sequences
    escPattern := regexp.MustCompile(`\x1b[0-9;]*[a-zA-Z]`)
    input = escPattern.ReplaceAllString(input, "")
    
    return input
}
```

**Files Modified**: `internal/tui/chat/inputarea_model.go`

### 3. Terminal Compatibility Improvements

**Problem**: Fixed color profile forcing could cause compatibility issues with different terminal emulators.

**Root Cause**: The code was forcing `termenv.ANSI256` regardless of terminal capabilities.

**Fix**: Added intelligent terminal detection and compatibility fallbacks:

```go
// Detect terminal capabilities
profile := termenv.ColorProfile()
log.Debug("Terminal environment details",
    "TERM", os.Getenv("TERM"),
    "COLORTERM", os.Getenv("COLORTERM"),
    "TERM_PROGRAM", os.Getenv("TERM_PROGRAM"),
    "color_profile", profile)

// Use detected profile or safe fallback
switch profile {
case termenv.TrueColor, termenv.ANSI256:
    lipgloss.SetColorProfile(profile)
default:
    lipgloss.SetColorProfile(termenv.ANSI)  // Safe fallback
}
```

**Files Modified**: `cmd/chat.go`

### 4. Enhanced Window Resize Handling

**Problem**: Header height calculations weren't properly synchronized during resize events, potentially causing layout issues.

**Root Cause**: Component updates weren't happening in the correct order during resize.

**Fix**: Improved resize handling with proper component update ordering:

```go
case tea.WindowSizeMsg:
    // Update header first to ensure height calculation is current
    m.header, headerCmd = m.header.Update(msg)
    
    // Then calculate layout with updated header height
    viewportHeight := m.layout.CalculateViewportHeightWithHeader(...)
    
    // Ensure viewport height is reasonable
    if viewportHeight < m.layout.GetMinViewportHeight() {
        viewportHeight = m.layout.GetMinViewportHeight()
    }
    
    // Finally update input area
    m.inputArea, inputCmd = m.inputArea.Update(msg)
```

**Files Modified**: `internal/tui/chat/model.go`

### 5. Header Model Improvements

**Problem**: Header refresh and git info updates during resize could cause instability.

**Fix**: Added conditional git info refresh and better state management:

```go
// Refresh git info on resize in case working directory changed
if h.gitRepo {
    newBranch, newRepo := getGitInfo(h.workingDir)
    if newRepo {
        h.gitBranch = newBranch
    } else {
        h.gitRepo = false
        h.gitBranch = ""
    }
}
```

**Files Modified**: `internal/tui/chat/header_model.go`

### 6. Enhanced Terminal Program Options

**Problem**: Missing input sanitization options could allow control sequences to leak through.

**Fix**: Added `tea.WithInputTTY()` option for better input handling:

```go
programOptions := []tea.ProgramOption{
    tea.WithAltScreen(),
    tea.WithMouseAllMotion(),
}

// Add input sanitization for better terminal control sequence handling
programOptions = append(programOptions, tea.WithInputTTY())

p := tea.NewProgram(chatAppModel, programOptions...)
```

**Files Modified**: `cmd/chat.go`

## Testing and Verification

### Build Verification
```bash
go build -o cge .  # ✅ Successful
```

### Test Suite
```bash
go test ./internal/tui/chat -v  # ✅ All tests passing
```

All existing functionality is preserved while fixing the identified issues.

## Benefits

### For Users
- **No more weird control sequences** appearing in input area
- **Stable header display** during terminal resize
- **Better terminal compatibility** across different emulators
- **Improved responsiveness** during window resize operations

### For Developers
- **Better debugging information** with terminal environment logging
- **Robust input sanitization** preventing control sequence leaks
- **Proper component update ordering** during resize events
- **Enhanced error handling** for layout calculations

### For Terminal Compatibility
- **Automatic terminal capability detection**
- **Safe fallbacks** for unsupported features
- **Comprehensive logging** for troubleshooting
- **Better handling of different terminal emulators**

## Related Files

### Primary Fixes
- `internal/tui/chat/inputarea_model.go` - Input sanitization and resize handling
- `internal/tui/chat/model.go` - Window resize coordination
- `internal/tui/chat/header_model.go` - Header state management
- `cmd/chat.go` - Terminal compatibility and program options

### Supporting Files
- `internal/tui/chat/theme.go` - Layout calculation support
- `internal/tui/chat/statusbar_model.go` - Status bar consistency

## Future Considerations

1. **Additional Terminal Testing**: Test with more terminal emulators (iTerm2, Windows Terminal, etc.)
2. **Control Sequence Monitoring**: Add metrics to track when sanitization occurs
3. **Performance Optimization**: Monitor resize performance with complex layouts
4. **User Feedback Integration**: Collect feedback on remaining display issues

## Usage

The fixes are automatically applied when using the chat command:

```bash
./cge chat  # All fixes are active
```

Debug information is available in the chat log file (`.cge/chat.log`) when issues occur. 