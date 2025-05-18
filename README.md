# codex-lite 🚀

<!-- Optional: Add a logo or a relevant image here -->
<!-- <img src="path/to/your/logo.png" alt="Codex Lite Logo" width="150" style="float: right;"> -->

**Codex Lite: Your AI-powered coding assistant.** It's a command-line tool that leverages local LLMs (via Ollama) to provide code explanation, analysis, and interactive chat capabilities. Built with Go, Cobra, and Bubble Tea for a rich TUI experience.

---

## 📌 Table of Contents

1.  [Overview](#1-overview)
2.  [Key Features](#2-key-features)
3.  [Project Structure](#3-project-structure-)
4.  [Getting Started](#4-getting-started)
    *   [Prerequisites](#prerequisites)
    *   [Installation](#installation)
    *   [Configuration](#configuration)
5.  [Usage](#5-usage)
    *   [Analyze Command](#analyze-command)
    *   [Chat Command](#chat-command)
6.  [Docker](#6-docker)
    *   [Building the Docker Image](#building-the-docker-image)
    *   [Running with Docker](#running-with-docker)
7.  [Contributing](#7-contributing)
8.  [License](#8-license)
9.  [Acknowledgements](#8-acknowledgements)

---

## **1️⃣ Overview**

**Codex Lite** is a versatile CLI tool designed to enhance your coding workflow by integrating with local Large Language Models (LLMs) through Ollama. It allows you to:
*   **Analyze your code:** Get insights, identify potential issues (syntax, smells, security vulnerabilities), and understand complex code segments.
*   **Chat with an LLM:** Engage in interactive conversations about your code or general programming topics.

The tool is built using Go and features a Terminal User Interface (TUI) powered by Bubble Tea for an enhanced user experience, especially for the chat and analysis results display. Configuration is flexible, supporting YAML files, environment variables, and command-line flags.

---

## **2️⃣ Key Features**

-   **AI-Powered Code Analysis:**
    -   Multiple analysis agents: `explain`, `syntax`, `smell`, `security`, `advanced`.
    -   Recursive file scanning with pattern matching and filtering by extension or directory.
    -   Customizable analysis depth and ignored directories.
    -   TUI for results display or plain text/JSON/Markdown/SARIF output.
-   **Interactive LLM Chat:**
    -   Engage in conversation with your chosen Ollama model.
    -   Session management: Continue previous chats and list available sessions.
    -   Chat history saved locally (`~/.codex-lite/chat_history/`).
    -   User-friendly TUI chat interface.
-   **Ollama Integration:** Seamlessly connects to your local Ollama instance.
-   **Flexible Configuration:**
    -   YAML configuration file (`$HOME/.codex-lite.yaml` or `./.codex-lite.yaml`).
    -   Environment variables (prefixed with `CODEXLITE_`).
    -   Command-line flags for overriding settings.
-   **Built with Go:** Efficient and portable.
-   **CLI and TUI:** Offers both a powerful command-line interface (via Cobra) and an optional rich terminal user interface (via Bubble Tea).

---

## **3️⃣ Project Structure 📁**

```bash
📂 codex-lite/
┣ 📂 cmd/                   # Cobra command definitions (root, analyze, chat)
┣ 📂 internal/               # Core application logic
┃ ┣ 📂 agents/             # Implementations for analysis agents (explain, syntax, etc.)
┃ ┣ 📂 config/             # Configuration loading (Viper)
┃ ┣ 📂 contextkeys/        # Keys for context values
┃ ┣ 📂 logger/             # Logging setup
┃ ┣ 📂 ollama/             # Ollama client logic
┃ ┣ 📂 orchestrator/       # Manages agent execution during analysis
┃ ┣ 📂 report/             # Formatting analysis output
┃ ┣ 📂 scanner/            # File system scanning logic
┃ ┗ 📂 tui/                # Bubble Tea components for Terminal User Interface
┣ 📄 .codex-lite.yaml       # Example configuration file
┣ 📄 .gitignore              # Git ignore rules
┣ 📄 Dockerfile              # For building Docker container
┣ 📄 .dockerignore           # Specifies files to exclude from Docker build context
┣ 📄 go.mod                 # Go module definition
┣ 📄 go.sum                 # Go module checksums
┣ 📄 LICENSE                # Project license (Please add one!)
┣ 📄 main.go                # Main application entry point
┗ 📄 README.md              # This file
```

---

## **4️⃣ Getting Started**

### **🔹 Prerequisites**

-   **Go:** Version 1.23 or higher (see `go.mod` for the exact version).
-   **Ollama:** A running Ollama instance with your desired models pulled (e.g., `ollama pull llama2`). Get it from [ollama.com](https://ollama.com/).
-   **(Optional) Docker:** If you plan to use the Docker image.

### **🔹 Installation**

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/castrovroberto/codex-lite.git
    cd codex-lite
    ```

2.  **Build the binary:**
    ```bash
    go build -o codex-lite main.go
    ```
    This will create an executable `codex-lite` in the current directory. You can move this to a directory in your `PATH` (e.g., `/usr/local/bin` or `~/bin`) for easier access.

    Alternatively, you can install directly using `go install`:
    ```bash
    go install github.com/castrovroberto/codex-lite@latest
    ```
    This will install the binary to your `$GOPATH/bin` or `$HOME/go/bin` directory.

### **🔹 Configuration**

Codex Lite can be configured in three ways (in order of precedence: flags > env vars > config file):

1.  **Configuration File:**
    Create a YAML file named `.codex-lite.yaml` in your home directory (`~/.codex-lite.yaml`) or the current project directory (`./.codex-lite.yaml`).
    You can copy and modify the provided `.codex-lite.yaml` as a starting point.

    Example `.codex-lite.yaml`:
    ```yaml
    # Ollama settings
    ollama_host_url: "http://localhost:11434" # URL of your Ollama instance
    default_model: "llama3:latest"           # Default model to use for chat and analysis
    ollama_request_timeout: "120s"           # Timeout for Ollama API requests
    ollama_keep_alive: "5m"                  # How long models stay loaded

    # Chat specific settings
    # Defines the AI's default behavior in chat by loading the specified file.
    # If empty or file not found, a default prompt is used.
    chat_system_prompt_file: "system-prompt.md"

    # Analysis settings (example)
    # max_concurrent_analyzers: 5
    # workspace_root: "."

    # Logging level (e.g., debug, info, warn, error)
    log_level: "info"
    ```

2.  **Environment Variables:**
    Set environment variables prefixed with `CODEXLITE_`.
    Example:
    ```bash
    export CODEXLITE_OLLAMA_HOST_URL="http://localhost:11434"
    export CODEXLITE_DEFAULT_MODEL="mistral"
    export CODEXLITE_CHAT_SYSTEM_PROMPT_FILE="path/to/your/system-prompt.md"
    export CODEXLITE_LOG_LEVEL="debug"
    ```

3.  **Command-line Flags:**
    Many configuration options can be overridden directly via command-line flags. See `codex-lite --help`, `codex-lite analyze --help`, and `codex-lite chat --help`.

    Persistent flags (apply to all commands):
    *   `--config FILE_PATH`: Path to the configuration file.
    *   `--ollama-host-url URL`: Ollama host URL.
    *   `--default-model MODEL_NAME`: Default LLM model.
    *   `--default-agent-list AGENT1,AGENT2`: Comma-separated list of default agents for analysis.

---

## **5️⃣ Usage**

Ensure Ollama is running and the desired models are available.

### **Analyze Command**

The `analyze` command processes specified code files or patterns using a suite of agents.

**Basic usage:**
```bash
codex-lite analyze [file_patterns...]
```

**Examples:**
```bash
# Analyze a single Go file with default agents
codex-lite analyze main.go

# Analyze all .py files in the current directory using only the 'explain' and 'syntax' agents
codex-lite analyze --agent explain,syntax "*.py"

# Recursively analyze all .js files in the 'src' directory, ignoring 'node_modules'
codex-lite analyze -r --ignore-dir node_modules "src/**/*.js"

# Analyze with a specific model
codex-lite analyze --default-model starcoder main.go

# Analyze without TUI and output to a JSON file
codex-lite analyze --no-tui --format json --output report.json "pkg/**/*.go"
```

**Key `analyze` flags:**
*   `--agent AGENT_NAME(S)` or `-a`: Comma-separated list of agents to use (e.g., `explain,smell`). If not set, uses agents from config or all available.
*   `--recursive` or `-r`: Scan directories recursively.
*   `--max-depth N`: Maximum depth for recursive scanning.
*   `--ignore-dir DIR_NAME`: Directory to ignore (can be specified multiple times).
*   `--ext .EXTENSION`: File extension to include (e.g., `.go`, `.py`; can be specified multiple times).
*   `--no-tui`: Disable TUI output and print to stdout.
*   `--output FILE_PATH` or `-o`: Output file for results (when `--no-tui` is used).
*   `--format FORMAT`: Output format when `--no-tui` is used (`text`, `json`, `markdown`, `sarif`). Defaults to `text`.

Run `codex-lite analyze --help` for all options.

### **Chat Command**

The `chat` command starts an interactive chat session with an LLM.

**Basic usage:**
```bash
codex-lite chat
```

**Examples:**
```bash
# Start a new chat session using the default model
codex-lite chat

# Start a chat session with a specific model
codex-lite chat --model mistral

# List available chat sessions
codex-lite chat --list-sessions

# Continue a previous chat session
codex-lite chat --session <session_id_from_list>
```

**Key `chat` flags:**
*   `--model MODEL_NAME` or `-m`: Model to use for the chat session (overrides default).
*   `--session SESSION_ID` or `-s`: Session ID to continue a previous chat.
*   `--list-sessions`: List available chat sessions.

Run `codex-lite chat --help` for all options. Chat history is saved in `~/.codex-lite/chat_history/`.

---

## **6️⃣ Docker**

You can build and run Codex Lite as a Docker container. This is useful for isolated environments or consistent deployments.

### **Building the Docker Image**

Ensure you have Docker installed. From the root of the project directory (where the `Dockerfile` is located):
```bash
docker build -t codex-lite-app .
```
You can tag the image differently if you prefer (e.g., `yourusername/codex-lite:latest`).

### **Running with Docker**

When running Codex Lite in Docker, you need to ensure it can communicate with your Ollama instance.

**1. If Ollama is running on your host machine:**
   You often need to use `host.docker.internal` to refer to your host machine from within the Docker container, or use host networking.

   **Using `host.docker.internal` (recommended for Mac/Windows Docker Desktop):**
   ```bash
   docker run -it --rm \
     codex-lite-app \
     --ollama-host-url="http://host.docker.internal:11434" chat --model your_model
   ```
   Replace `your_model` with a model you have pulled in Ollama.

   **Using host networking (Linux):**
   This makes the container share the host's network stack.
   ```bash
   docker run -it --rm --network="host" \
     codex-lite-app \
     chat --model your_model
   ```
   If using host networking, `codex-lite` inside the container can usually connect to `http://localhost:11434` if Ollama is listening on all interfaces or on `localhost` on the host. You might still need to pass `--ollama-host-url="http://localhost:11434"` if the default in the app or its config doesn't align.

**2. Mounting local configuration and history (optional but recommended for persistence):**
   To persist chat history and use a local configuration file:
   ```bash
   # Create directories on host if they don't exist
   mkdir -p $HOME/.codex-lite/chat_history
   # Ensure your .codex-lite.yaml is in $HOME/.codex-lite.yaml or provide its path

   docker run -it --rm \
     -v "$HOME/.codex-lite:/home/appuser/.codex-lite" \
     codex-lite-app \
     --config="/home/appuser/.codex-lite/.codex-lite.yaml" \
     --ollama-host-url="http://host.docker.internal:11434" \
     chat
   ```
   *Note:* The `Dockerfile` creates a non-root user `appuser`. The application looks for config in `$HOME/.codex-lite.yaml` which inside the container for `appuser` is `/home/appuser/.codex-lite.yaml`.

**3. Analyzing local files:**
   To analyze files from your host machine, you need to mount the relevant directory into the container.
   ```bash
   docker run -it --rm \
     -v "$(pwd):/src" \
     -w /src \
     codex-lite-app \
     --ollama-host-url="http://host.docker.internal:11434" \
     analyze "your_file.go"
   ```
   This mounts the current host directory (`$(pwd)`) to `/src` inside the container and sets `/src` as the working directory.

**Important Considerations for Docker:**
*   **Ollama Accessibility:** The most common issue is the Docker container not being able to reach Ollama. Ensure your Ollama instance is configured to accept connections from the Docker container's IP or from `host.docker.internal`. If Ollama is also in Docker, they might need to be on the same Docker network.
*   **Models:** The LLM models themselves reside within your Ollama instance, not in the `codex-lite-app` Docker image.

---

## **7️⃣ Contributing**

Contributions are welcome! If you'd like to contribute:
1.  Fork the Project (`https://github.com/castrovroberto/codex-lite/fork`)
2.  Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3.  Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4.  Push to the Branch (`git push origin feature/AmazingFeature`)
5.  Open a Pull Request

Please ensure your code adheres to Go best practices and that tests pass. Adding new tests for new features is highly encouraged.

---

## **8️⃣ License**

Distributed under the **MIT License**. See `LICENSE` for more information.
*(You currently have an empty `LICENSE` file. Please choose and add a license, e.g., MIT, Apache 2.0.)*

---

## **9️⃣ Acknowledgements**

*   **Ollama Team:** For making local LLM hosting accessible.
*   **Charmbracelet Team:** For `bubbletea`, `lipgloss`, and other fantastic TUI libraries.
*   **spf13:** For `cobra` and `viper`.

