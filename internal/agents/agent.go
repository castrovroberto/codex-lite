package agents

type Result struct {
    File    string
    Output  string
    Agent   string
}

type Agent interface {
    Name() string
    Analyze(path string, code string) (Result, error)
}
