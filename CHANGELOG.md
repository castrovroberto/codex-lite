# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Project Refactoring (CGE - Phase 0):**
  - Began transformation from `CGE` to `Codex-GPT-Engineer (CGE)`.
  - Introduced `codex.toml` as the new TOML-based configuration file.
  - Refactored `internal/config/config.go` to support `codex.toml`, updated `AppConfig` structure for CGE, and set new environment variable prefix `CGE_`.
  - Initial update of `README.md` to reflect CGE vision and new configuration.
- **Chat:** Configurable system prompt loaded from an external file (e.g., `system-prompt.md`) via `chat_system_prompt_file` in config.
- **Chat:** Live "thinking" timer displayed in the status bar while waiting for AI response.
- **Chat:** Implemented saving of chat history on exit to `~/.cge/chat_history/`.
- **Chat:** Improved accuracy of `StartTime` and `EndTime` in saved chat history.
- **Chat:** Session ID format updated to be more filename-friendly.
- **Project:** Comprehensive `README.md` overhaul with detailed setup, usage, and contribution guidelines.
- **Project:** Docker support with `Dockerfile` and `.dockerignore` for containerized builds and execution.
- **Chat:** Display AI response processing time after each message. 