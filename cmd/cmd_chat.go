//go:build darwin && arm64 && !nomlx

package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"forge.lthn.ai/core/cli/pkg/cli"
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
	chatModelPath  string
	chatOutput     string
	chatKB         string
	chatKernel     string
	chatSystem     string
	chatMaxTokens  int
	chatTemp       float64
	chatMemLimit   int
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

	fmt.Println("Chat started. Type /quit to exit, /help for commands.")
	if sandwich {
		fmt.Println("Sandwich signing enabled (KB + kernel)")
	}
	if chatOutput != "" {
		fmt.Printf("Capturing to: %s\n", chatOutput)
	}
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB input buffer

	for {
		fmt.Print("you> ")
		if !scanner.Scan() {
			// EOF (Ctrl+D)
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			cmd := strings.Fields(input)
			switch cmd[0] {
			case "/quit", "/exit":
				goto done
			case "/save":
				if chatOutput == "" {
					fmt.Println("No --output file specified. Use --output to enable saving.")
					continue
				}
				if len(history) > 0 {
					savedConversations = append(savedConversations, cloneMessages(history))
					fmt.Printf("Saved conversation (%d messages)\n", len(history))
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
				fmt.Println("Conversation cleared.")
				continue
			case "/system":
				if len(cmd) < 2 {
					fmt.Println("Usage: /system <prompt text>")
					continue
				}
				sysText := strings.TrimPrefix(input, "/system ")
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
				fmt.Printf("System prompt set (%d chars)\n", len(sysText))
				continue
			case "/undo":
				// Remove last user+assistant pair
				if len(history) >= 2 {
					last := history[len(history)-1]
					secondLast := history[len(history)-2]
					if secondLast.Role == "user" && last.Role == "assistant" {
						history = history[:len(history)-2]
						fmt.Println("Last exchange removed.")
					} else {
						fmt.Println("Cannot undo: last messages are not a user/assistant pair.")
					}
				} else {
					fmt.Println("Nothing to undo.")
				}
				continue
			case "/help":
				fmt.Println("Commands:")
				fmt.Println("  /quit, /exit    End session and save")
				fmt.Println("  /save           Save conversation so far")
				fmt.Println("  /clear          Clear conversation history")
				fmt.Println("  /system <text>  Set system prompt")
				fmt.Println("  /undo           Remove last exchange")
				fmt.Println("  /help           Show this help")
				continue
			default:
				fmt.Printf("Unknown command: %s (try /help)\n", cmd[0])
				continue
			}
		}

		// Add user message
		history = append(history, ml.Message{Role: "user", Content: input})

		// Generate response
		genStart := time.Now()
		fmt.Print("\nassistant> ")

		var response strings.Builder
		err := backend.ChatStream(cmd.Context(), history, opts, func(token string) error {
			fmt.Print(token)
			response.WriteString(token)
			return nil
		})
		fmt.Println()

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

		fmt.Println()
	}

done:
	fmt.Println()

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

	encoder := json.NewEncoder(f)
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

		if err := encoder.Encode(record); err != nil {
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
