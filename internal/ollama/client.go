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

func Query(model, prompt string) (string, error) {
    body, _ := json.Marshal(Request{
        Model:  model,
        Prompt: prompt,
        Stream: false,
    })
    resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(body))
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var result Response
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Response, nil
}
