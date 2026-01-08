package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/gYonder/drime-shell/internal/commands"
	"github.com/gYonder/drime-shell/internal/session"
)

// CommandChain represents a sequence of pipelines connected by &&, ||, or ;
type CommandChain struct {
	Commands []ChainedPipeline
}

// ChainedPipeline is a pipeline with the operator connecting it to the next pipeline
type ChainedPipeline struct {
	Pipeline *Pipeline
	Operator ChainOperator // operator AFTER this pipeline
}

// Pipeline represents a parsed command line with optional piping and redirection.
type Pipeline struct {
	Segments []*Segment
}

// Segment is a single command in a pipeline with optional redirection.
type Segment struct {
	Args         []string
	CommandName  string
	InputFile    string // < file
	OutputFile   string // > or >> file
	ErrorFile    string // 2> or 2>> file
	AppendOutput bool   // >> instead of >
	AppendError  bool   // 2>> instead of 2>
	MergeStderr  bool   // 2>&1
}

// ParseCommandChain parses a command line into a CommandChain structure.
// This handles &&, ||, ; operators as well as pipes and redirections.
func ParseCommandChain(line string) (*CommandChain, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	tokens, err := Tokenize(line)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, nil
	}

	// Split by chain operators (&&, ||, ;)
	chainedCmds := SplitByChain(tokens)

	chain := &CommandChain{}
	for _, cc := range chainedCmds {
		if len(cc.Tokens) == 0 {
			if cc.Operator != ChainNone {
				continue // Empty command before operator, skip
			}
			continue
		}

		pipeline, err := parsePipelineFromTokens(cc.Tokens)
		if err != nil {
			return nil, err
		}

		chain.Commands = append(chain.Commands, ChainedPipeline{
			Pipeline: pipeline,
			Operator: cc.Operator,
		})
	}

	if len(chain.Commands) == 0 {
		return nil, nil
	}

	return chain, nil
}

// parsePipelineFromTokens parses tokens into a Pipeline (handles pipes and redirections)
func parsePipelineFromTokens(tokens []Token) (*Pipeline, error) {
	segments := SplitByPipe(tokens)
	pipeline := &Pipeline{}

	for i, segTokens := range segments {
		if len(segTokens) == 0 {
			return nil, fmt.Errorf("syntax error near unexpected token `|'")
		}
		seg, err := parseSegment(segTokens, i == 0, i == len(segments)-1)
		if err != nil {
			return nil, err
		}
		pipeline.Segments = append(pipeline.Segments, seg)
	}
	return pipeline, nil
}

// ParsePipeline parses a command line into a Pipeline structure (legacy, for single pipelines).
func ParsePipeline(line string) (*Pipeline, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	tokens, err := Tokenize(line)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, nil
	}

	return parsePipelineFromTokens(tokens)
}

// parseSegment extracts command, args, and redirections from tokens.
func parseSegment(tokens []Token, isFirst, isLast bool) (*Segment, error) {
	seg := &Segment{}
	var cmdTokens []Token

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		switch tok.Type {
		case TokenWord:
			cmdTokens = append(cmdTokens, tok)

		case TokenRedirectIn:
			if !isFirst {
				return nil, fmt.Errorf("input redirection '<' only allowed on first command in pipeline")
			}
			file, err := expectFilename(tokens, i, "<")
			if err != nil {
				return nil, err
			}
			seg.InputFile = file
			i++

		case TokenRedirectOut, TokenRedirectAppend:
			if !isLast {
				return nil, fmt.Errorf("output redirection '%s' only allowed on last command in pipeline", tok.Value)
			}
			file, err := expectFilename(tokens, i, tok.Value)
			if err != nil {
				return nil, err
			}
			seg.OutputFile = file
			seg.AppendOutput = tok.Type == TokenRedirectAppend
			i++

		case TokenRedirectErr, TokenRedirectErrAppend:
			if !isLast {
				return nil, fmt.Errorf("error redirection '%s' only allowed on last command in pipeline", tok.Value)
			}
			file, err := expectFilename(tokens, i, tok.Value)
			if err != nil {
				return nil, err
			}
			seg.ErrorFile = file
			seg.AppendError = tok.Type == TokenRedirectErrAppend
			i++

		case TokenRedirectAll:
			if !isLast {
				return nil, fmt.Errorf("combined redirection '&>' only allowed on last command in pipeline")
			}
			file, err := expectFilename(tokens, i, "&>")
			if err != nil {
				return nil, err
			}
			seg.OutputFile = file
			seg.MergeStderr = true
			i++

		case TokenRedirectErrToOut:
			seg.MergeStderr = true
		}
	}

	if len(cmdTokens) == 0 {
		return nil, fmt.Errorf("syntax error: empty command")
	}

	seg.CommandName = cmdTokens[0].Value
	for _, tok := range cmdTokens[1:] {
		seg.Args = append(seg.Args, tok.Value)
	}
	return seg, nil
}

func expectFilename(tokens []Token, i int, op string) (string, error) {
	if i+1 >= len(tokens) || tokens[i+1].Type != TokenWord {
		return "", fmt.Errorf("syntax error: missing filename after '%s'", op)
	}
	return tokens[i+1].Value, nil
}

// Execute runs the command chain, respecting &&, ||, and ; semantics.
func (c *CommandChain) Execute(ctx context.Context, sess *session.Session) error {
	if c == nil || len(c.Commands) == 0 {
		return nil
	}

	var lastErr error
	for i, cp := range c.Commands {
		// Determine whether to run this command based on previous result
		shouldRun := true
		if i > 0 {
			prevOp := c.Commands[i-1].Operator
			switch prevOp {
			case ChainAnd:
				// Run only if previous succeeded
				shouldRun = lastErr == nil
			case ChainOr:
				// Run only if previous failed
				shouldRun = lastErr != nil
			case ChainSeq:
				// Always run
				shouldRun = true
			}
		}

		if !shouldRun {
			continue
		}

		lastErr = cp.Pipeline.Execute(ctx, sess)
	}

	return lastErr
}

// Execute runs the pipeline.
func (p *Pipeline) Execute(ctx context.Context, sess *session.Session) error {
	if p == nil || len(p.Segments) == 0 {
		return nil
	}

	// Resolve all commands upfront
	cmds := make([]*commands.Command, len(p.Segments))
	for i, seg := range p.Segments {
		cmd, ok := commands.Get(seg.CommandName)
		if !ok {
			return fmt.Errorf("command not found: %s", seg.CommandName)
		}
		cmds[i] = cmd
	}

	if len(p.Segments) == 1 {
		return p.executeSingle(ctx, sess, cmds[0], p.Segments[0])
	}
	return p.executePipeline(ctx, sess, cmds)
}

// executeSingle runs a single command with redirection.
func (p *Pipeline) executeSingle(ctx context.Context, sess *session.Session, cmd *commands.Command, seg *Segment) error {
	env, closers, err := setupRedirection(ctx, sess, seg)
	if err != nil {
		return err
	}

	// Expand globs
	expandedArgs, err := ExpandGlobs(ctx, sess, env.Stderr, seg.Args)
	if err != nil {
		closeAll(closers)
		return err
	}

	// Check for -h/--help flag
	if commands.HasHelpFlag(expandedArgs) {
		commands.PrintUsage(cmd, env.Stdout)
		closeAll(closers)
		return nil
	}

	runErr := cmd.Run(ctx, sess, env, expandedArgs)

	// Close all redirects - this is where uploads happen!
	closeErr := closeAllWithError(closers)

	// Return command error first, then close error
	if runErr != nil {
		return runErr
	}
	return closeErr
}

// executePipeline runs multiple commands connected by pipes.
func (p *Pipeline) executePipeline(ctx context.Context, sess *session.Session, cmds []*commands.Command) error {
	n := len(p.Segments)

	envs := make([]*commands.ExecutionEnv, n)
	for i := range envs {
		envs[i] = &commands.ExecutionEnv{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	}

	var closers []io.Closer
	defer closeAll(closers)

	// Create pipes between commands
	for i := 0; i < n-1; i++ {
		pr, pw, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe: %v", err)
		}
		closers = append(closers, pr, pw)
		envs[i].Stdout = pw
		envs[i+1].Stdin = pr
	}

	// Input redirection on first command
	if file := p.Segments[0].InputFile; file != "" {
		rfr, err := NewRemoteFileReader(ctx, sess, file)
		if err != nil {
			return fmt.Errorf("%s: %v", file, err)
		}
		closers = append(closers, rfr)
		envs[0].Stdin = rfr
	}

	// Output/error redirection on last command
	lastEnv := envs[n-1]
	lastSeg := p.Segments[n-1]
	if err := applyOutputRedirection(ctx, sess, lastSeg, lastEnv, &closers); err != nil {
		return err
	}

	// Run all commands concurrently
	var wg sync.WaitGroup
	errors := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer closePipeWriter(envs[idx])

			// Expand globs
			expandedArgs, err := ExpandGlobs(ctx, sess, envs[idx].Stderr, p.Segments[idx].Args)
			if err != nil {
				errors[idx] = err
				return
			}

			// Check for -h/--help flag
			if commands.HasHelpFlag(expandedArgs) {
				commands.PrintUsage(cmds[idx], envs[idx].Stdout)
				return
			}
			errors[idx] = cmds[idx].Run(ctx, sess, envs[idx], expandedArgs)
		}(i)
	}
	wg.Wait()

	for i, err := range errors {
		if err != nil {
			return fmt.Errorf("%s: %v", p.Segments[i].CommandName, err)
		}
	}
	return nil
}

// setupRedirection creates an ExecutionEnv with proper I/O redirection.
func setupRedirection(ctx context.Context, sess *session.Session, seg *Segment) (*commands.ExecutionEnv, []io.Closer, error) {
	env := &commands.ExecutionEnv{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	var closers []io.Closer

	// Input redirection
	if seg.InputFile != "" {
		rfr, err := NewRemoteFileReader(ctx, sess, seg.InputFile)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %v", seg.InputFile, err)
		}
		closers = append(closers, rfr)
		env.Stdin = rfr
	}

	// Output/error redirection
	if err := applyOutputRedirection(ctx, sess, seg, env, &closers); err != nil {
		closeAll(closers)
		return nil, nil, err
	}

	return env, closers, nil
}

// applyOutputRedirection handles >, >>, 2>, 2>>, 2>&1, &>
func applyOutputRedirection(ctx context.Context, sess *session.Session, seg *Segment, env *commands.ExecutionEnv, closers *[]io.Closer) error {
	// Stdout redirection
	if seg.OutputFile != "" {
		w, err := openOutputWriter(ctx, sess, seg.OutputFile, seg.AppendOutput)
		if err != nil {
			return fmt.Errorf("%s: %v", seg.OutputFile, err)
		}
		if c, ok := w.(io.Closer); ok {
			*closers = append(*closers, c)
		}
		env.Stdout = w
	}

	// Handle 2>&1
	if seg.MergeStderr {
		env.Stderr = env.Stdout
	}

	// Stderr redirection (only if not merged)
	if seg.ErrorFile != "" && !seg.MergeStderr {
		w, err := openOutputWriter(ctx, sess, seg.ErrorFile, seg.AppendError)
		if err != nil {
			return fmt.Errorf("%s: %v", seg.ErrorFile, err)
		}
		if c, ok := w.(io.Closer); ok {
			*closers = append(*closers, c)
		}
		env.Stderr = w
	}

	return nil
}

// openOutputWriter returns a writer for the given path, handling /dev/null.
func openOutputWriter(ctx context.Context, sess *session.Session, path string, append bool) (io.Writer, error) {
	if path == "/dev/null" || path == "dev/null" {
		return devNull{}, nil
	}
	return NewRemoteFileWriterWithMode(ctx, sess, path, append)
}

type devNull struct{}

func (devNull) Write(p []byte) (int, error) { return len(p), nil }
func (devNull) Close() error                { return nil }

func closeAll(closers []io.Closer) {
	for _, c := range closers {
		c.Close()
	}
}

// closeAllWithError closes all closers and returns the first error encountered.
// This is important for RemoteFileWriter which uploads on Close().
func closeAllWithError(closers []io.Closer) error {
	var firstErr error
	for _, c := range closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func closePipeWriter(env *commands.ExecutionEnv) {
	if env.Stdout != os.Stdout {
		if c, ok := env.Stdout.(io.Closer); ok {
			c.Close()
		}
	}
}
