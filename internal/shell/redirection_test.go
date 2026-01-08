package shell_test

import (
	"strings"
	"testing"

	"github.com/gYonder/drime-shell/internal/shell"
)

// ============================================================================
// TOKENIZER TESTS
// ============================================================================

func TestTokenize_BasicCommands(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []shell.Token
	}{
		{
			name:  "simple command",
			input: "echo hello",
			expected: []shell.Token{
				{Value: "echo", Type: shell.TokenWord},
				{Value: "hello", Type: shell.TokenWord},
			},
		},
		{
			name:  "command with multiple args",
			input: "ls -la /path/to/dir",
			expected: []shell.Token{
				{Value: "ls", Type: shell.TokenWord},
				{Value: "-la", Type: shell.TokenWord},
				{Value: "/path/to/dir", Type: shell.TokenWord},
			},
		},
		{
			name:  "single quoted string",
			input: "echo 'hello world'",
			expected: []shell.Token{
				{Value: "echo", Type: shell.TokenWord},
				{Value: "hello world", Type: shell.TokenWord, Quoted: true},
			},
		},
		{
			name:  "double quoted string",
			input: `echo "hello world"`,
			expected: []shell.Token{
				{Value: "echo", Type: shell.TokenWord},
				{Value: "hello world", Type: shell.TokenWord, Quoted: true},
			},
		},
		{
			name:  "escaped space",
			input: `echo hello\ world`,
			expected: []shell.Token{
				{Value: "echo", Type: shell.TokenWord},
				{Value: "hello world", Type: shell.TokenWord},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := shell.Tokenize(tt.input)
			if err != nil {
				t.Fatalf("Tokenize(%q) error: %v", tt.input, err)
			}
			if len(tokens) != len(tt.expected) {
				t.Fatalf("Tokenize(%q) got %d tokens, want %d\nGot: %+v", tt.input, len(tokens), len(tt.expected), tokens)
			}
			for i, tok := range tokens {
				if tok.Value != tt.expected[i].Value || tok.Type != tt.expected[i].Type {
					t.Errorf("Token[%d] = {%q, %v}, want {%q, %v}",
						i, tok.Value, tok.Type, tt.expected[i].Value, tt.expected[i].Type)
				}
			}
		})
	}
}

func TestTokenize_Pipes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []shell.Token
	}{
		{
			name:  "simple pipe",
			input: "cat file | sort",
			expected: []shell.Token{
				{Value: "cat", Type: shell.TokenWord},
				{Value: "file", Type: shell.TokenWord},
				{Value: "|", Type: shell.TokenPipe},
				{Value: "sort", Type: shell.TokenWord},
			},
		},
		{
			name:  "pipe without spaces",
			input: "cat file|sort",
			expected: []shell.Token{
				{Value: "cat", Type: shell.TokenWord},
				{Value: "file", Type: shell.TokenWord},
				{Value: "|", Type: shell.TokenPipe},
				{Value: "sort", Type: shell.TokenWord},
			},
		},
		{
			name:  "multiple pipes",
			input: "cat file | sort | uniq | head",
			expected: []shell.Token{
				{Value: "cat", Type: shell.TokenWord},
				{Value: "file", Type: shell.TokenWord},
				{Value: "|", Type: shell.TokenPipe},
				{Value: "sort", Type: shell.TokenWord},
				{Value: "|", Type: shell.TokenPipe},
				{Value: "uniq", Type: shell.TokenWord},
				{Value: "|", Type: shell.TokenPipe},
				{Value: "head", Type: shell.TokenWord},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := shell.Tokenize(tt.input)
			if err != nil {
				t.Fatalf("Tokenize(%q) error: %v", tt.input, err)
			}
			if len(tokens) != len(tt.expected) {
				t.Fatalf("Tokenize(%q) got %d tokens, want %d\nGot: %+v", tt.input, len(tokens), len(tt.expected), tokens)
			}
			for i, tok := range tokens {
				if tok.Value != tt.expected[i].Value || tok.Type != tt.expected[i].Type {
					t.Errorf("Token[%d] = {%q, %v}, want {%q, %v}",
						i, tok.Value, tok.Type, tt.expected[i].Value, tt.expected[i].Type)
				}
			}
		})
	}
}

func TestTokenize_OutputRedirection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []shell.Token
	}{
		{
			name:  "stdout redirect overwrite",
			input: "echo hello > file.txt",
			expected: []shell.Token{
				{Value: "echo", Type: shell.TokenWord},
				{Value: "hello", Type: shell.TokenWord},
				{Value: ">", Type: shell.TokenRedirectOut},
				{Value: "file.txt", Type: shell.TokenWord},
			},
		},
		{
			name:  "stdout redirect append",
			input: "echo hello >> file.txt",
			expected: []shell.Token{
				{Value: "echo", Type: shell.TokenWord},
				{Value: "hello", Type: shell.TokenWord},
				{Value: ">>", Type: shell.TokenRedirectAppend},
				{Value: "file.txt", Type: shell.TokenWord},
			},
		},
		{
			name:  "redirect without spaces",
			input: "echo hello>file.txt",
			expected: []shell.Token{
				{Value: "echo", Type: shell.TokenWord},
				{Value: "hello", Type: shell.TokenWord},
				{Value: ">", Type: shell.TokenRedirectOut},
				{Value: "file.txt", Type: shell.TokenWord},
			},
		},
		{
			name:  "redirect to quoted filename",
			input: `echo hello > "my file.txt"`,
			expected: []shell.Token{
				{Value: "echo", Type: shell.TokenWord},
				{Value: "hello", Type: shell.TokenWord},
				{Value: ">", Type: shell.TokenRedirectOut},
				{Value: "my file.txt", Type: shell.TokenWord, Quoted: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := shell.Tokenize(tt.input)
			if err != nil {
				t.Fatalf("Tokenize(%q) error: %v", tt.input, err)
			}
			if len(tokens) != len(tt.expected) {
				t.Fatalf("Tokenize(%q) got %d tokens, want %d\nGot: %+v", tt.input, len(tokens), len(tt.expected), tokens)
			}
			for i, tok := range tokens {
				if tok.Value != tt.expected[i].Value || tok.Type != tt.expected[i].Type {
					t.Errorf("Token[%d] = {%q, %v}, want {%q, %v}",
						i, tok.Value, tok.Type, tt.expected[i].Value, tt.expected[i].Type)
				}
			}
		})
	}
}

func TestTokenize_InputRedirection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []shell.Token
	}{
		{
			name:  "stdin redirect",
			input: "sort < file.txt",
			expected: []shell.Token{
				{Value: "sort", Type: shell.TokenWord},
				{Value: "<", Type: shell.TokenRedirectIn},
				{Value: "file.txt", Type: shell.TokenWord},
			},
		},
		{
			name:  "stdin redirect without spaces",
			input: "sort<file.txt",
			expected: []shell.Token{
				{Value: "sort", Type: shell.TokenWord},
				{Value: "<", Type: shell.TokenRedirectIn},
				{Value: "file.txt", Type: shell.TokenWord},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := shell.Tokenize(tt.input)
			if err != nil {
				t.Fatalf("Tokenize(%q) error: %v", tt.input, err)
			}
			if len(tokens) != len(tt.expected) {
				t.Fatalf("Tokenize(%q) got %d tokens, want %d\nGot: %+v", tt.input, len(tokens), len(tt.expected), tokens)
			}
			for i, tok := range tokens {
				if tok.Value != tt.expected[i].Value || tok.Type != tt.expected[i].Type {
					t.Errorf("Token[%d] = {%q, %v}, want {%q, %v}",
						i, tok.Value, tok.Type, tt.expected[i].Value, tt.expected[i].Type)
				}
			}
		})
	}
}

func TestTokenize_StderrRedirection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []shell.Token
	}{
		{
			name:  "stderr redirect overwrite",
			input: "cmd 2> errors.txt",
			expected: []shell.Token{
				{Value: "cmd", Type: shell.TokenWord},
				{Value: "2>", Type: shell.TokenRedirectErr},
				{Value: "errors.txt", Type: shell.TokenWord},
			},
		},
		{
			name:  "stderr redirect append",
			input: "cmd 2>> errors.txt",
			expected: []shell.Token{
				{Value: "cmd", Type: shell.TokenWord},
				{Value: "2>>", Type: shell.TokenRedirectErrAppend},
				{Value: "errors.txt", Type: shell.TokenWord},
			},
		},
		{
			name:  "stderr to stdout",
			input: "cmd 2>&1",
			expected: []shell.Token{
				{Value: "cmd", Type: shell.TokenWord},
				{Value: "2>&1", Type: shell.TokenRedirectErrToOut},
			},
		},
		{
			name:  "stderr redirect without spaces",
			input: "cmd 2>errors.txt",
			expected: []shell.Token{
				{Value: "cmd", Type: shell.TokenWord},
				{Value: "2>", Type: shell.TokenRedirectErr},
				{Value: "errors.txt", Type: shell.TokenWord},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := shell.Tokenize(tt.input)
			if err != nil {
				t.Fatalf("Tokenize(%q) error: %v", tt.input, err)
			}
			if len(tokens) != len(tt.expected) {
				t.Fatalf("Tokenize(%q) got %d tokens, want %d\nGot: %+v", tt.input, len(tokens), len(tt.expected), tokens)
			}
			for i, tok := range tokens {
				if tok.Value != tt.expected[i].Value || tok.Type != tt.expected[i].Type {
					t.Errorf("Token[%d] = {%q, %v}, want {%q, %v}",
						i, tok.Value, tok.Type, tt.expected[i].Value, tt.expected[i].Type)
				}
			}
		})
	}
}

func TestTokenize_CombinedRedirection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []shell.Token
	}{
		{
			name:  "redirect all with &>",
			input: "cmd &> all.txt",
			expected: []shell.Token{
				{Value: "cmd", Type: shell.TokenWord},
				{Value: "&>", Type: shell.TokenRedirectAll},
				{Value: "all.txt", Type: shell.TokenWord},
			},
		},
		{
			name:  "redirect all with >&",
			input: "cmd >& all.txt",
			expected: []shell.Token{
				{Value: "cmd", Type: shell.TokenWord},
				{Value: ">&", Type: shell.TokenRedirectAll},
				{Value: "all.txt", Type: shell.TokenWord},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := shell.Tokenize(tt.input)
			if err != nil {
				t.Fatalf("Tokenize(%q) error: %v", tt.input, err)
			}
			if len(tokens) != len(tt.expected) {
				t.Fatalf("Tokenize(%q) got %d tokens, want %d\nGot: %+v", tt.input, len(tokens), len(tt.expected), tokens)
			}
			for i, tok := range tokens {
				if tok.Value != tt.expected[i].Value || tok.Type != tt.expected[i].Type {
					t.Errorf("Token[%d] = {%q, %v}, want {%q, %v}",
						i, tok.Value, tok.Type, tt.expected[i].Value, tt.expected[i].Type)
				}
			}
		})
	}
}

func TestTokenize_ComplexCombinations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []shell.Token
	}{
		{
			name:  "stdout and stderr to same file",
			input: "cmd > out.txt 2>&1",
			expected: []shell.Token{
				{Value: "cmd", Type: shell.TokenWord},
				{Value: ">", Type: shell.TokenRedirectOut},
				{Value: "out.txt", Type: shell.TokenWord},
				{Value: "2>&1", Type: shell.TokenRedirectErrToOut},
			},
		},
		{
			name:  "stdout and stderr to different files",
			input: "cmd > out.txt 2> err.txt",
			expected: []shell.Token{
				{Value: "cmd", Type: shell.TokenWord},
				{Value: ">", Type: shell.TokenRedirectOut},
				{Value: "out.txt", Type: shell.TokenWord},
				{Value: "2>", Type: shell.TokenRedirectErr},
				{Value: "err.txt", Type: shell.TokenWord},
			},
		},
		{
			name:  "pipe with output redirect",
			input: "cat file | sort > sorted.txt",
			expected: []shell.Token{
				{Value: "cat", Type: shell.TokenWord},
				{Value: "file", Type: shell.TokenWord},
				{Value: "|", Type: shell.TokenPipe},
				{Value: "sort", Type: shell.TokenWord},
				{Value: ">", Type: shell.TokenRedirectOut},
				{Value: "sorted.txt", Type: shell.TokenWord},
			},
		},
		{
			name:  "input and output redirect",
			input: "sort < input.txt > output.txt",
			expected: []shell.Token{
				{Value: "sort", Type: shell.TokenWord},
				{Value: "<", Type: shell.TokenRedirectIn},
				{Value: "input.txt", Type: shell.TokenWord},
				{Value: ">", Type: shell.TokenRedirectOut},
				{Value: "output.txt", Type: shell.TokenWord},
			},
		},
		{
			name:  "pipe with input redirect on first command",
			input: "sort < input.txt | uniq > output.txt",
			expected: []shell.Token{
				{Value: "sort", Type: shell.TokenWord},
				{Value: "<", Type: shell.TokenRedirectIn},
				{Value: "input.txt", Type: shell.TokenWord},
				{Value: "|", Type: shell.TokenPipe},
				{Value: "uniq", Type: shell.TokenWord},
				{Value: ">", Type: shell.TokenRedirectOut},
				{Value: "output.txt", Type: shell.TokenWord},
			},
		},
		{
			name:  "dev null redirect",
			input: "cmd 2>/dev/null",
			expected: []shell.Token{
				{Value: "cmd", Type: shell.TokenWord},
				{Value: "2>", Type: shell.TokenRedirectErr},
				{Value: "/dev/null", Type: shell.TokenWord},
			},
		},
		{
			name:  "silence all output",
			input: "cmd > /dev/null 2>&1",
			expected: []shell.Token{
				{Value: "cmd", Type: shell.TokenWord},
				{Value: ">", Type: shell.TokenRedirectOut},
				{Value: "/dev/null", Type: shell.TokenWord},
				{Value: "2>&1", Type: shell.TokenRedirectErrToOut},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := shell.Tokenize(tt.input)
			if err != nil {
				t.Fatalf("Tokenize(%q) error: %v", tt.input, err)
			}
			if len(tokens) != len(tt.expected) {
				t.Fatalf("Tokenize(%q) got %d tokens, want %d\nGot: %+v", tt.input, len(tokens), len(tt.expected), tokens)
			}
			for i, tok := range tokens {
				if tok.Value != tt.expected[i].Value || tok.Type != tt.expected[i].Type {
					t.Errorf("Token[%d] = {%q, %v}, want {%q, %v}",
						i, tok.Value, tok.Type, tt.expected[i].Value, tt.expected[i].Type)
				}
			}
		})
	}
}

func TestTokenize_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unclosed single quote", "echo 'hello"},
		{"unclosed double quote", `echo "hello`},
		{"trailing backslash", `echo hello\`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := shell.Tokenize(tt.input)
			if err == nil {
				t.Errorf("Tokenize(%q) expected error, got nil", tt.input)
			}
		})
	}
}

// ============================================================================
// PIPELINE PARSING TESTS
// ============================================================================

func TestParsePipeline_SingleCommand(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		commandName  string
		args         []string
		inputFile    string
		outputFile   string
		errorFile    string
		appendOutput bool
		appendError  bool
		mergeStderr  bool
	}{
		{
			name:        "simple command",
			input:       "echo hello world",
			commandName: "echo",
			args:        []string{"hello", "world"},
		},
		{
			name:        "output redirect overwrite",
			input:       "echo hello > out.txt",
			commandName: "echo",
			args:        []string{"hello"},
			outputFile:  "out.txt",
		},
		{
			name:         "output redirect append",
			input:        "echo hello >> out.txt",
			commandName:  "echo",
			args:         []string{"hello"},
			outputFile:   "out.txt",
			appendOutput: true,
		},
		{
			name:        "input redirect",
			input:       "sort < input.txt",
			commandName: "sort",
			args:        []string{},
			inputFile:   "input.txt",
		},
		{
			name:        "stderr redirect",
			input:       "cmd 2> err.txt",
			commandName: "cmd",
			args:        []string{},
			errorFile:   "err.txt",
		},
		{
			name:        "stderr append",
			input:       "cmd 2>> err.txt",
			commandName: "cmd",
			args:        []string{},
			errorFile:   "err.txt",
			appendError: true,
		},
		{
			name:        "merge stderr to stdout",
			input:       "cmd 2>&1",
			commandName: "cmd",
			args:        []string{},
			mergeStderr: true,
		},
		{
			name:        "stdout redirect with merge",
			input:       "cmd > out.txt 2>&1",
			commandName: "cmd",
			args:        []string{},
			outputFile:  "out.txt",
			mergeStderr: true,
		},
		{
			name:        "combined redirect &>",
			input:       "cmd &> all.txt",
			commandName: "cmd",
			args:        []string{},
			outputFile:  "all.txt",
			mergeStderr: true,
		},
		{
			name:        "input and output redirect",
			input:       "sort < in.txt > out.txt",
			commandName: "sort",
			args:        []string{},
			inputFile:   "in.txt",
			outputFile:  "out.txt",
		},
		{
			name:        "all three redirects",
			input:       "cmd < in.txt > out.txt 2> err.txt",
			commandName: "cmd",
			args:        []string{},
			inputFile:   "in.txt",
			outputFile:  "out.txt",
			errorFile:   "err.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline, err := shell.ParsePipeline(tt.input)
			if err != nil {
				t.Fatalf("ParsePipeline(%q) error: %v", tt.input, err)
			}
			if pipeline == nil {
				t.Fatalf("ParsePipeline(%q) returned nil", tt.input)
			}
			if len(pipeline.Segments) != 1 {
				t.Fatalf("Expected 1 segment, got %d", len(pipeline.Segments))
			}

			seg := pipeline.Segments[0]
			if seg.CommandName != tt.commandName {
				t.Errorf("CommandName = %q, want %q", seg.CommandName, tt.commandName)
			}
			if len(seg.Args) != len(tt.args) {
				t.Errorf("Args = %v, want %v", seg.Args, tt.args)
			} else {
				for i, arg := range seg.Args {
					if arg != tt.args[i] {
						t.Errorf("Args[%d] = %q, want %q", i, arg, tt.args[i])
					}
				}
			}
			if seg.InputFile != tt.inputFile {
				t.Errorf("InputFile = %q, want %q", seg.InputFile, tt.inputFile)
			}
			if seg.OutputFile != tt.outputFile {
				t.Errorf("OutputFile = %q, want %q", seg.OutputFile, tt.outputFile)
			}
			if seg.ErrorFile != tt.errorFile {
				t.Errorf("ErrorFile = %q, want %q", seg.ErrorFile, tt.errorFile)
			}
			if seg.AppendOutput != tt.appendOutput {
				t.Errorf("AppendOutput = %v, want %v", seg.AppendOutput, tt.appendOutput)
			}
			if seg.AppendError != tt.appendError {
				t.Errorf("AppendError = %v, want %v", seg.AppendError, tt.appendError)
			}
			if seg.MergeStderr != tt.mergeStderr {
				t.Errorf("MergeStderr = %v, want %v", seg.MergeStderr, tt.mergeStderr)
			}
		})
	}
}

func TestParsePipeline_MultiplePipes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []struct {
			commandName string
			args        []string
		}
	}{
		{
			name:  "two commands",
			input: "cat file.txt | sort",
			expected: []struct {
				commandName string
				args        []string
			}{
				{"cat", []string{"file.txt"}},
				{"sort", []string{}},
			},
		},
		{
			name:  "three commands",
			input: "cat file.txt | sort | uniq",
			expected: []struct {
				commandName string
				args        []string
			}{
				{"cat", []string{"file.txt"}},
				{"sort", []string{}},
				{"uniq", []string{}},
			},
		},
		{
			name:  "four commands with args",
			input: "cat file.txt | sort -r | uniq -c | head -n 10",
			expected: []struct {
				commandName string
				args        []string
			}{
				{"cat", []string{"file.txt"}},
				{"sort", []string{"-r"}},
				{"uniq", []string{"-c"}},
				{"head", []string{"-n", "10"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline, err := shell.ParsePipeline(tt.input)
			if err != nil {
				t.Fatalf("ParsePipeline(%q) error: %v", tt.input, err)
			}
			if len(pipeline.Segments) != len(tt.expected) {
				t.Fatalf("Got %d segments, want %d", len(pipeline.Segments), len(tt.expected))
			}

			for i, seg := range pipeline.Segments {
				exp := tt.expected[i]
				if seg.CommandName != exp.commandName {
					t.Errorf("Segment[%d].CommandName = %q, want %q", i, seg.CommandName, exp.commandName)
				}
				if len(seg.Args) != len(exp.args) {
					t.Errorf("Segment[%d].Args = %v, want %v", i, seg.Args, exp.args)
				}
			}
		})
	}
}

func TestParsePipeline_PipeWithRedirection(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		numSegs    int
		firstInput string
		lastOutput string
	}{
		{
			name:       "pipe with input on first",
			input:      "sort < in.txt | uniq",
			numSegs:    2,
			firstInput: "in.txt",
		},
		{
			name:       "pipe with output on last",
			input:      "cat file | sort > out.txt",
			numSegs:    2,
			lastOutput: "out.txt",
		},
		{
			name:       "pipe with both",
			input:      "sort < in.txt | uniq > out.txt",
			numSegs:    2,
			firstInput: "in.txt",
			lastOutput: "out.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline, err := shell.ParsePipeline(tt.input)
			if err != nil {
				t.Fatalf("ParsePipeline(%q) error: %v", tt.input, err)
			}
			if len(pipeline.Segments) != tt.numSegs {
				t.Fatalf("Got %d segments, want %d", len(pipeline.Segments), tt.numSegs)
			}

			if pipeline.Segments[0].InputFile != tt.firstInput {
				t.Errorf("First segment InputFile = %q, want %q",
					pipeline.Segments[0].InputFile, tt.firstInput)
			}
			if pipeline.Segments[len(pipeline.Segments)-1].OutputFile != tt.lastOutput {
				t.Errorf("Last segment OutputFile = %q, want %q",
					pipeline.Segments[len(pipeline.Segments)-1].OutputFile, tt.lastOutput)
			}
		})
	}
}

func TestParsePipeline_Errors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		errContains string
	}{
		{
			name:        "input redirect on non-first command",
			input:       "cat file | sort < input.txt",
			errContains: "only allowed on first",
		},
		{
			name:        "output redirect on non-last command",
			input:       "cat > out.txt | sort",
			errContains: "only allowed on last",
		},
		{
			name:        "missing filename after >",
			input:       "echo hello >",
			errContains: "missing filename",
		},
		{
			name:        "missing filename after <",
			input:       "sort <",
			errContains: "missing filename",
		},
		{
			name:        "empty command in pipe",
			input:       "cat file | | sort",
			errContains: "unexpected token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := shell.ParsePipeline(tt.input)
			if err == nil {
				t.Errorf("ParsePipeline(%q) expected error, got nil", tt.input)
				return
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("ParsePipeline(%q) error = %q, want to contain %q",
					tt.input, err.Error(), tt.errContains)
			}
		})
	}
}

// ============================================================================
// REAL WORLD SCENARIOS
// ============================================================================

func TestRealWorldScenarios_Tokenize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Common Unix patterns
		{"sort with pipe to wc", "cat file.txt | sort | wc -l", false},
		{"find with uniq", "find . -name '*.go' | sort | uniq", false},
		{"silent stderr", "cmd 2>/dev/null", false},
		{"silent all", "cmd >/dev/null 2>&1", false},
		{"log stderr", "cmd 2>> error.log", false},
		{"tee pattern", "cmd | tee output.txt", false},
		{"complex pipeline", "cat data.csv | sort -t',' -k2 | uniq | head -20 > result.txt", false},

		// Edge cases
		{"redirect in quotes", `echo ">" file`, false},
		{"pipe in quotes", `echo "hello | world"`, false},
		{"multiple spaces", "echo    hello    world", false},
		{"tabs", "echo\thello\tworld", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := shell.Tokenize(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for %q", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Tokenize(%q) error: %v", tt.input, err)
				}
				if len(tokens) == 0 {
					t.Errorf("Tokenize(%q) returned empty tokens", tt.input)
				}
			}
		})
	}
}

func TestRealWorldScenarios_Parse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Log file analysis
		{"filter errors", "cat app.log | sort | uniq", false},
		{"count lines", "cat app.log | wc -l", false},
		{"save sorted", "cat app.log | sort > sorted.txt", false},

		// Data processing
		{"sort csv", "sort -t',' -k2 data.csv > sorted.csv", false},
		{"unique lines", "sort data.txt | uniq > unique.txt", false},
		{"count unique", "sort data.txt | uniq -c | sort -rn | head -10", false},

		// Redirection patterns
		{"overwrite file", "echo 'new content' > file.txt", false},
		{"append to file", "echo 'more content' >> file.txt", false},
		{"separate streams", "cmd > stdout.txt 2> stderr.txt", false},
		{"combined streams", "cmd &> all.txt", false},

		// Silent execution
		{"silent errors", "cmd 2>/dev/null", false},
		{"completely silent", "cmd >/dev/null 2>&1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline, err := shell.ParsePipeline(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for %q", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("ParsePipeline(%q) error: %v", tt.input, err)
				}
				if pipeline == nil || len(pipeline.Segments) == 0 {
					t.Errorf("ParsePipeline(%q) returned empty pipeline", tt.input)
				}
			}
		})
	}
}

func TestEmptyInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tabs only", "\t\t"},
		{"mixed whitespace", "  \t  \t  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline, err := shell.ParsePipeline(tt.input)
			if err != nil {
				t.Errorf("ParsePipeline(%q) error: %v", tt.input, err)
			}
			if pipeline != nil && len(pipeline.Segments) > 0 {
				t.Errorf("ParsePipeline(%q) expected nil or empty, got %d segments",
					tt.input, len(pipeline.Segments))
			}
		})
	}
}

// ============================================================================
// /dev/null HANDLING TESTS
// ============================================================================

func TestDevNullParsing(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		outputFile string
		errorFile  string
	}{
		{
			name:      "stderr to dev null",
			input:     "cmd 2>/dev/null",
			errorFile: "/dev/null",
		},
		{
			name:       "stdout to dev null",
			input:      "cmd >/dev/null",
			outputFile: "/dev/null",
		},
		{
			name:       "both to dev null via &>",
			input:      "cmd &>/dev/null",
			outputFile: "/dev/null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline, err := shell.ParsePipeline(tt.input)
			if err != nil {
				t.Fatalf("ParsePipeline(%q) error: %v", tt.input, err)
			}
			if pipeline == nil || len(pipeline.Segments) == 0 {
				t.Fatalf("ParsePipeline(%q) returned nil", tt.input)
			}

			seg := pipeline.Segments[0]
			if seg.OutputFile != tt.outputFile {
				t.Errorf("OutputFile = %q, want %q", seg.OutputFile, tt.outputFile)
			}
			if seg.ErrorFile != tt.errorFile {
				t.Errorf("ErrorFile = %q, want %q", seg.ErrorFile, tt.errorFile)
			}
		})
	}
}

// ============================================================================
// EDGE CASES
// ============================================================================

func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid edge cases
		{"single command", "ls", false},
		{"command with only flags", "ls -la", false},
		{"quoted empty string", `echo ""`, false},
		{"escaped quotes", `echo "hello \"world\""`, false},
		{"nested quotes", `echo 'hello "world"'`, false},

		// File paths with special chars
		{"file with dots", "cat file.name.txt", false},
		{"file with dashes", "cat my-file.txt", false},
		{"file with underscores", "cat my_file.txt", false},
		{"absolute path", "cat /path/to/file.txt", false},
		{"relative path", "cat ./file.txt", false},
		{"parent path", "cat ../file.txt", false},
		{"home path", "cat ~/file.txt", false},

		// Complex redirections
		{"multiple args then redirect", "echo one two three > file.txt", false},
		{"redirect then args (invalid)", "echo > file.txt hello", false}, // This is actually parsed but behaves differently
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := shell.ParsePipeline(tt.input)
			if tt.wantErr && err == nil {
				t.Errorf("ParsePipeline(%q) expected error, got nil", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ParsePipeline(%q) unexpected error: %v", tt.input, err)
			}
		})
	}
}

// Ensure token types are exported and usable
var (
	_ = shell.TokenWord
	_ = shell.TokenPipe
	_ = shell.TokenRedirectOut
	_ = shell.TokenRedirectAppend
	_ = shell.TokenRedirectIn
	_ = shell.TokenRedirectErr
	_ = shell.TokenRedirectErrAppend
	_ = shell.TokenRedirectAll
	_ = shell.TokenRedirectErrToOut
)
