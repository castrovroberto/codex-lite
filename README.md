# Codex-GPT-Engineer (CGE) 🚀

<!-- Optional: Add a logo or a relevant image here -->
<!-- <img src="path/to/your/logo.png" alt="CGE Logo" width="150" style="float: right;"> -->

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/castrovroberto/CGE)

**Codex-GPT-Engineer (CGE): Your AI-powered partner for engineering complex software projects.** CGE is evolving from `CGE` into a sophisticated command-line tool that leverages LLMs (via Ollama or OpenAI) to assist with project planning, code generation, automated reviews, and knowledge graph integration.

**Note:** This project is currently undergoing a significant refactoring from its `CGE` origins to the new CGE architecture. Some features described may be in development.

---

##  Table of Contents

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

**Codex-GPT-Engineer (CGE)** is a versatile CLI tool designed to enhance your software engineering workflow. It integrates with Large Language Models (LLMs) like those accessible via Ollama and OpenAI to:
*   **Plan Projects:** Generate comprehensive development plans based on your requirements.
*   **Generate Code:** Create new files or modify existing ones based on the generated plan.
*   **Automated Review & Iteration:** Validate generated code using tests and linters, and iteratively refine it with LLM assistance.
*   **Knowledge Graph Memory (KGM):** Leverage a knowledge graph to provide deeper context and memory to the LLM over time.
*   **(Future) TUI:** Offer an intuitive Terminal User Interface for managing the CGE workflow.

The tool is built using Go. Configuration is managed via `codex.toml`, environment variables, and command-line flags.

---

## **2️⃣ Key Features**

-   **LLM-Driven Project Planning:** Generates structured `plan.json` from high-level goals.
-   **Automated Code Generation & Modification:** Applies changes based on the plan, with support for dry-runs and diffs.
-   **Interactive Review Loop:** Executes tests/linters, feeds results back to LLM for corrections.
-   **Knowledge Graph Memory (KGM):** (Upcoming) Stores and retrieves project context using a graph database.
-   **Multi-Provider LLM Support:**
    -   Ollama for local models.
    -   OpenAI for access to powerful cloud models (with budget tracking).
-   **Flexible Configuration:**
    -   TOML configuration file (`codex.toml`).
    -   Environment variables (prefixed with `CGE_`).
    -   Command-line flags for overriding settings.
-   **Built with Go:** Efficient and portable.
-   **Evolving CLI:** Commands being refactored to `plan`, `generate`, `diff`, `review`, `run`, `kg query`.
-   **(Future) Rich TUI:** For an enhanced user experience.

---

## **3️⃣ Project Structure 📁**

```bash
📂 cge/ (formerly CGE)
┣ 📂 cmd/                   # Cobra command definitions (e.g., plan, generate, review)
┣ 📂 internal/               # Core application logic
┃ ┣ 📂 config/             # Configuration loading (Viper, codex.toml)
┃ ┣ 📂 llm/                # LLM client interfaces and implementations (Ollama, OpenAI)
┃ ┣ 📂 kgm/                # Knowledge Graph Memory client and logic (Upcoming)
┃ ┣ 📂 contextkeys/        # Keys for context values
┃ ┣ 📂 logger/             # Logging setup
┃ ┣ 📂 scanner/            # File system scanning logic
┃ ┗ 📂 tui/                # Bubble Tea components for TUI (To be adapted)
// ... (other CGE-specific internal packages as they are developed, e.g., planner, generator, reviewer)
┣ 📄 codex.toml             # Main configuration file (TOML format)
┣ 📄 .gitignore              # Git ignore rules
┣ 📄 Dockerfile              # For building Docker container (To be reviewed/updated)
┣ 📄 .dockerignore           # Specifies files to exclude from Docker build context
┣ 📄 go.mod                 # Go module definition
┣ 📄 go.sum                 # Go module checksums
┣ 📄 LICENSE                # Project license
┣ 📄 main.go                # Main application entry point
┗ 📄 README.md              # This file
```

---

## **4️⃣ Getting Started**

### **🔹 Prerequisites**

-   **Go:** Version 1.23 or higher (see `go.mod` for the exact version).
-   **LLM Provider:**
    -   **Ollama:** A running Ollama instance with your desired models pulled (e.g., `ollama pull llama3`). Get it from [ollama.com](https://ollama.com/).
    -   **OpenAI:** (Optional) An OpenAI API key if you plan to use OpenAI models.
-   **(Optional) Docker:** If you plan to use the Docker image.

### **🔹 Installation**

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/castrovroberto/CGE.git cge # Consider renaming the directory
    cd cge
    ```

2.  **Build the binary (example name `cge`):**
    ```bash
    go build -o cge main.go
    ```
    This will create an executable `cge` in the current directory. You can move this to a directory in your `PATH`.

    Alternatively, you can install directly using `go install` (ensure your `go.mod` reflects the new project name if it changes):
    ```bash
    # If module path is updated in go.mod, use that path here
    # go install github.com/your-username/cge@latest
    ```

### **🔹 Configuration**

CGE can be configured in three ways (in order of precedence: flags > env vars > config file):

1.  **Configuration File (`codex.toml`):**
    Create a TOML file named `codex.toml` in your home directory (`~/.cge/codex.toml` or `~/.codex.toml`) or the current project directory (`./codex.toml`).
    An example `codex.toml` is provided in the repository.

    Example `codex.toml` (refer to the actual `codex.toml` in the repo for more details):
    ```toml
    version = "0.1.0"

    [llm]
      provider = "ollama"  # "ollama" or "openai"
      model = "llama3"
      # request_timeout_seconds = "300s"

    [logging]
      level = "info"
    ```

2.  **Environment Variables:**
    Set environment variables prefixed with `CGE_`. For nested keys in `codex.toml`, use an underscore (e.g., `CGE_LLM_PROVIDER`).
    Example:
    ```bash
    export CGE_LLM_PROVIDER="openai"
    export CGE_LLM_MODEL="gpt-4-turbo"
    export OPENAI_API_KEY="your_openai_api_key" # Note: OPENAI_API_KEY is a special case
    export CGE_LOGGING_LEVEL="debug"
    ```

3.  **Command-line Flags:**
    Many configuration options can be overridden directly via command-line flags. See `cge --help` and help for specific subcommands (e.g., `cge plan --help`).

    Persistent flags (apply to all commands - to be updated as CGE evolves):
    *   `--config FILE_PATH`: Path to the `codex.toml` configuration file.
    *   `--llm-provider PROVIDER_NAME`: (Example) LLM Provider (ollama, openai).
    *   `--llm-model MODEL_NAME`: (Example) LLM model.

---

## **5️⃣ Usage**

Ensure your chosen LLM provider (Ollama or OpenAI) is configured and running.

**Note:** The commands below reflect the target CGE structure and are under development.

### **`plan` Command**
Generates a `plan.json` file detailing the steps to achieve a given software development goal.
```bash
cge plan --goal "Refactor the authentication module to use JWT"
```

### **`generate` Command**
Reads `plan.json` and interacts with the LLM to generate or modify code.
```bash
cge generate
cge generate --dry-run # To see proposed changes
```

### **`diff` Command**
Shows differences and allows interactive application of generated patches.
```bash
cge diff
```

### **`review` Command**
Executes tests/linters, feeds results back to LLM for patch revisions.
```bash
cge review
```

### **(Legacy) `chat` Command**
The `chat` command from `CGE` may be retained or adapted.
```bash
cge chat
```

Run `cge <command> --help` for all options.

---

## **6️⃣ Docker**

You can build and run Codex Lite as a Docker container. This is useful for isolated environments or consistent deployments.

### **Building the Docker Image**

Ensure you have Docker installed. From the root of the project directory (where the `Dockerfile` is located):
```bash
docker build -t cge-app .
```
You can tag the image differently if you prefer (e.g., `yourusername/CGE:latest`).

### **Running with Docker**

When running Codex Lite in Docker, you need to ensure it can communicate with your Ollama instance.

**1. If Ollama is running on your host machine:**
   You often need to use `host.docker.internal` to refer to your host machine from within the Docker container, or use host networking.

   **Using `host.docker.internal` (recommended for Mac/Windows Docker Desktop):**
   ```bash
   docker run -it --rm \
     cge-app \
     --ollama-host-url="http://host.docker.internal:11434" chat --model your_model
   ```
   Replace `your_model` with a model you have pulled in Ollama.

   **Using host networking (Linux):**
   This makes the container share the host's network stack.
   ```bash
   docker run -it --rm --network="host" \
     cge-app \
     chat --model your_model
   ```
   If using host networking, `CGE` inside the container can usually connect to `http://localhost:11434` if Ollama is listening on all interfaces or on `localhost` on the host. You might still need to pass `--ollama-host-url="http://localhost:11434"` if the default in the app or its config doesn't align.

**2. Mounting local configuration and history (optional but recommended for persistence):**
   To persist chat history and use a local configuration file:
   ```bash
   # Create directories on host if they don't exist
   mkdir -p $HOME/.cge/chat_history
   # Ensure your .cge.yaml is in $HOME/.cge.yaml or provide its path

   docker run -it --rm \
     -v "$HOME/.cge:/home/appuser/.cge" \
     cge-app \
     --config="/home/appuser/.cge/.cge.yaml" \
     --ollama-host-url="http://host.docker.internal:11434" \
     chat
   ```
   *Note:* The `Dockerfile` creates a non-root user `appuser`. The application looks for config in `$HOME/.cge.yaml` which inside the container for `appuser` is `/home/appuser/.cge.yaml`.

**3. Analyzing local files:**
   To analyze files from your host machine, you need to mount the relevant directory into the container.
   ```bash
   docker run -it --rm \
     -v "$(pwd):/src" \
     -w /src \
     cge-app \
     --ollama-host-url="http://host.docker.internal:11434" \
     analyze "your_file.go"
   ```
   This mounts the current host directory (`$(pwd)`) to `/src` inside the container and sets `/src` as the working directory.

**Important Considerations for Docker:**
*   **Ollama Accessibility:** The most common issue is the Docker container not being able to reach Ollama. Ensure your Ollama instance is configured to accept connections from the Docker container's IP or from `host.docker.internal`. If Ollama is also in Docker, they might need to be on the same Docker network.
*   **Models:** The LLM models themselves reside within your Ollama instance, not in the `cge-app` Docker image.

---

## **7️⃣ Contributing**

Contributions are welcome! If you'd like to contribute:
1.  Fork the Project (`https://github.com/castrovroberto/CGE/fork`)
2.  Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3.  Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4.  Push to the Branch (`git push origin feature/AmazingFeature`)
5.  Open a Pull Request

Please ensure your code adheres to Go best practices and that tests pass. Adding new tests for new features is highly encouraged.

---

## **8️⃣ License**

Distributed under the **MIT License**. See `LICENSE` for more information.

---

## **9️⃣ Acknowledgements**

*   **Ollama Team:** For making local LLM hosting accessible.
*   **Charmbracelet Team:** For `bubbletea`, `lipgloss`, and other fantastic TUI libraries.
*   **spf13:** For `cobra` and `viper`.

