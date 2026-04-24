//go:build darwin && arm64 && !nomlx && cliv1

package cmd

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"runtime"
	"time"

	"dappco.re/go/core"
	"dappco.re/go/cli/pkg/cli"
	coreio "dappco.re/go/io"
	coreerr "dappco.re/go/log"
	"dappco.re/go/ml"
)

var chatCmd = &cli.Command{
	Use:   "chat",
	Short: "Interactive conversation with a local MLX model",
	Long: `Start an interactive chat session with a local MLX model.

All exchanges are captured and can be written to training JSONL on exit
for use with 'core ml train'. Optionally apply axiom sandwich signing
to wrap the conversation for LEK training.

Commands during chat:
  /quit, /exit    End session and save
  /save           Save conversation so far (appends to output)
  /clear          Clear conversation history
  /system <text>  Set system prompt
  /undo           Remove last exchange`,
	RunE: runChat,
}

var (
	chatModelPath string
	chatOutput    string
	chatKB        string
	chatKernel    string
	chatSystem    string
	chatMaxTokens int
	chatTemp      float64
	chatMemLimit  int
)

func init() {
	chatCmd.Flags().StringVar(&chatModelPath, "model-path", "", "Path to model directory (required)")
	chatCmd.Flags().StringVar(&chatOutput, "output", "", "Output JSONL file for captured conversation")
	chatCmd.Flags().StringVar(&chatKB, "kb", "", "Knowledge base document for sandwich signing")
	chatCmd.Flags().StringVar(&chatKernel, "kernel", "", "LEK-1 kernel file for sandwich signing")
	chatCmd.Flags().StringVar(&chatSystem, "system", "", "Initial system prompt")
	chatCmd.Flags().IntVar(&chatMaxTokens, "max-tokens", 2048, "Max tokens per response")
	chatCmd.Flags().Float64Var(&chatTemp, "temperature", 0.4, "Sampling temperature")
	chatCmd.Flags().IntVar(&chatMemLimit, "memory-limit", 24, "Metal memory limit in GB")
	chatCmd.MarkFlagRequired("model-path")
}

func runChat(cmd *cli.Command, args []string) error {
	// Load optional KB and kernel for sandwich signing
	var kbText, kernelText string
	if chatKB != "" {
		data, err := coreio.Local.Read(chatKB)
		if err != nil {
			return coreerr.E("cmd.runChat", "read KB", err)
		}
		kbText = data
	}
	if chatKernel != "" {
		data, err := coreio.Local.Read(chatKernel)
		if err != nil {
			return coreerr.E("cmd.runChat", "read kernel", err)
		}
		kernelText = data
	}
	sandwich := kbText != "" && kernelText != ""

	// Load model
	slog.Info("chat: loading model", "path", chatModelPath)
	backend, err := ml.NewMLXBackend(chatModelPath)
	if err != nil {
		return coreerr.E("cmd.runChat", "load model", err)
	}

	opts := ml.GenOpts{
		Temperature: chatTemp,
		MaxTokens:   chatMaxTokens,
	}

	// Conversation state
	var history []ml.Message
	if chatSystem != "" {
		history = append(history, ml.Message{Role: "system", Content: chatSystem})
	}

	// Track saved conversations for JSONL output
	var savedConversations [][]ml.Message
	out := cmd.OutOrStdout()

	core.Print(out, "Chat started. Type /quit to exit, /help for commands.")
	if sandwich {
		core.Print(out, "Sandwich signing enabled (KB + kernel)")
	}
	if chatOutput != "" {
		core.Print(out, "Capturing to: %s", chatOutput)
	}
	core.Print(out, "")

	scanner := bufio.NewScanner(cmd.InOrStdin())
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB input buffer

	for {
		if err := writeChatText(out, "you> "); err != nil {
			return coreerr.E("cmd.runChat", "write prompt", err)
		}
		if !scanner.Scan() {
			// EOF (Ctrl+D)
			break
		}

		input := core.Trim(scanner.Text())
		if input == "" {
			continue
		}

		// Handle commands
		if core.HasPrefix(input, "/") {
			parts := chatFields(input)
			switch parts[0] {
			case "/quit", "/exit":
				goto done
			case "/save":
				if chatOutput == "" {
					core.Print(out, "No --output file specified. Use --output to enable saving.")
					continue
				}
				if len(history) > 0 {
					savedConversations = append(savedConversations, cloneMessages(history))
					core.Print(out, "Saved conversation (%d messages)", len(history))
				}
				continue
			case "/clear":
				sysPrompt := ""
				for _, m := range history {
					if m.Role == "system" {
						sysPrompt = m.Content
						break
					}
				}
				history = nil
				if sysPrompt != "" {
					history = append(history, ml.Message{Role: "system", Content: sysPrompt})
				}
				core.Print(out, "Conversation cleared.")
				continue
			case "/system":
				if len(parts) < 2 {
					core.Print(out, "Usage: /system <prompt text>")
					continue
				}
				sysText := core.TrimPrefix(input, "/system ")
				// Replace existing system prompt or add new one
				found := false
				for i, m := range history {
					if m.Role == "system" {
						history[i].Content = sysText
						found = true
						break
					}
				}
				if !found {
					// Prepend system message
					history = append([]ml.Message{{Role: "system", Content: sysText}}, history...)
				}
				core.Print(out, "System prompt set (%d chars)", len(sysText))
				continue
			case "/undo":
				// Remove last user+assistant pair
				if len(history) >= 2 {
					last := history[len(history)-1]
					secondLast := history[len(history)-2]
					if secondLast.Role == "user" && last.Role == "assistant" {
						history = history[:len(history)-2]
						core.Print(out, "Last exchange removed.")
					} else {
						core.Print(out, "Cannot undo: last messages are not a user/assistant pair.")
					}
				} else {
					core.Print(out, "Nothing to undo.")
				}
				continue
			case "/help":
				core.Print(out, "Commands:")
				core.Print(out, "  /quit, /exit    End session and save")
				core.Print(out, "  /save           Save conversation so far")
				core.Print(out, "  /clear          Clear conversation history")
				core.Print(out, "  /system <text>  Set system prompt")
				core.Print(out, "  /undo           Remove last exchange")
				core.Print(out, "  /help           Show this help")
				continue
			default:
				core.Print(out, "Unknown command: %s (try /help)", parts[0])
				continue
			}
		}

		// Add user message
		history = append(history, ml.Message{Role: "user", Content: input})

		// Generate response
		genStart := time.Now()
		if err := writeChatText(out, "\nassistant> "); err != nil {
			return coreerr.E("cmd.runChat", "write assistant prompt", err)
		}

		response := core.NewBuilder()
		err := backend.ChatStream(cmd.Context(), history, opts, func(token string) error {
			if err := writeChatText(out, token); err != nil {
				return err
			}
			_, _ = response.WriteString(token)
			return nil
		})
		core.Print(out, "")

		if err != nil {
			slog.Error("chat: generation failed", "error", err)
			// Remove the failed user message
			history = history[:len(history)-1]
			continue
		}

		elapsed := time.Since(genStart)
		responseText := response.String()
		history = append(history, ml.Message{Role: "assistant", Content: responseText})

		slog.Debug("chat: response generated",
			"chars", len(responseText),
			"duration", elapsed.Round(time.Millisecond),
		)

		// Periodic cleanup
		if len(history)%8 == 0 {
			runtime.GC()
		}

		core.Print(out, "")
	}

done:
	core.Print(out, "")

	// Save final conversation if output is specified
	if chatOutput != "" && len(history) > 0 {
		// Include current conversation if not already saved
		savedConversations = append(savedConversations, history)

		if err := writeChatJSONL(chatOutput, savedConversations, sandwich, kbText, kernelText); err != nil {
			return coreerr.E("cmd.runChat", "save conversation", err)
		}
	}

	return nil
}

// writeChatJSONL writes conversations to JSONL file.
// If sandwich is true, wraps user messages with KB + kernel signing.
func writeChatJSONL(path string, conversations [][]ml.Message, sandwich bool, kb, kernel string) error {
	f, err := coreio.Local.Append(path)
	if err != nil {
		return err
	}
	defer f.Close()
	written := 0

	for _, conv := range conversations {
		// Extract user/assistant pairs (skip system messages for training output)
		var messages []ml.Message
		for _, m := range conv {
			if m.Role == "system" {
				continue
			}
			messages = append(messages, m)
		}

		if len(messages) < 2 {
			continue
		}

		if sandwich {
			// Apply sandwich signing to user messages
			messages = applySandwichSigning(messages, kb, kernel)
		}

		record := struct {
			Messages []ml.Message `json:"messages"`
		}{Messages: messages}

		if _, err := io.WriteString(f, core.Concat(core.JSONMarshalString(record), "\n")); err != nil {
			return err
		}
		written++
	}

	slog.Info("chat: saved conversations",
		"file", path,
		"conversations", written,
		"sandwich", sandwich,
	)
	return nil
}

// applySandwichSigning wraps user messages with KB preamble and kernel postfix.
func applySandwichSigning(messages []ml.Message, kb, kernel string) []ml.Message {
	signed := make([]ml.Message, len(messages))
	copy(signed, messages)

	for i := range signed {
		if signed[i].Role == "user" {
			signed[i].Content = buildSandwich(kb, signed[i].Content, kernel)
		}
	}
	return signed
}

// cloneMessages creates a deep copy of a message slice.
func cloneMessages(msgs []ml.Message) []ml.Message {
	clone := make([]ml.Message, len(msgs))
	copy(clone, msgs)
	return clone
}

func chatFields(input string) []string {
	raw := bytes.Fields([]byte(input))
	out := make([]string, len(raw))
	for i := range raw {
		out[i] = string(raw[i])
	}
	return out
}

func writeChatText(w io.Writer, text string) error {
	_, err := io.WriteString(w, text)
	return err
}
