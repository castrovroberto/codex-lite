package ollama

import (
    "bytes"
    "encoding/json"
    "net/http"
)

type Request struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
    Stream bool   `json:"stream"`
}

type Response struct {
    Response string `json:"response"`
}

func Query(ollamaHostURL string, modelName string, prompt string) (string, error) {
    reqBody := Request{
        Model:  modelName,
        Prompt: prompt,
        Stream: false,
    }
    body, err := json.Marshal(reqBody)
    if err != nil {
        return "", fmt.Errorf("failed to marshal ollama request: %w", err)
    }

    resp, err := http.Post(fmt.Sprintf("%s/api/generate", ollamaHostURL), "application/json", bytes.NewBuffer(body))
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var result Response
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("failed to decode ollama response: %w", err)
    }
    return result.Response, nil
}
