package models

type CodeSubmission struct {
	SourceCode             string `json:"source_code"`
	LanguageID             int    `json:"language_id"`
	Stdin                  string `json:"stdin,omitempty"`
	CompilerOptions        string `json:"compiler_options,omitempty"`
	CommandLineArguments   string `json:"command_line_arguments,omitempty"`
	RedirectStderrToStdout bool   `json:"redirect_stderr_to_stdout,omitempty"`
}

type CodeSubmissionResult struct {
	Token         string     `json:"token,omitempty"`
	Stdout        string     `json:"stdout,omitempty"`
	Stderr        string     `json:"stderr,omitempty"`
	CompileOutput string     `json:"compile_output,omitempty"`
	Message       string     `json:"message,omitempty"`
	Status        CodeStatus `json:"status,omitempty"`
	Time          string     `json:"time,omitempty"`
	Memory        int        `json:"memory,omitempty"`
	RunTime       string     `json:"runtime,omitempty"`
}

type CodeStatus struct {
	ID          int    `json:"id,omitempty"`
	Description string `json:"description,omitempty"`
}
