package inference

import goai "github.com/rcarmo/go-ai"

// Tool input is an object even for no-argument tools. The pinned go-ai
// Anthropic decoder leaves the map nil when no argument delta arrives; its
// encoder would send input:null on the next request. Installed Pi starts at {}.
// Copy only modified slices so normalisation cannot mutate caller history.
func objectToolInputs(message *goai.Message) *goai.Message {
	if message == nil {
		return nil
	}
	var out *goai.Message
	for i, block := range message.Content {
		if block.Type != "toolCall" || block.Arguments != nil {
			continue
		}
		if out == nil {
			copyMessage := *message
			copyMessage.Content = append([]goai.ContentBlock(nil), message.Content...)
			out = &copyMessage
		}
		out.Content[i].Arguments = map[string]any{}
	}
	if out != nil {
		return out
	}
	return message
}
func contextWithObjectToolInputs(input *goai.Context) *goai.Context {
	if input == nil {
		return nil
	}
	var out *goai.Context
	for i := range input.Messages {
		message := objectToolInputs(&input.Messages[i])
		if message == &input.Messages[i] {
			continue
		}
		if out == nil {
			copyContext := *input
			copyContext.Messages = append([]goai.Message(nil), input.Messages...)
			out = &copyContext
		}
		out.Messages[i] = *message
	}
	if out != nil {
		return out
	}
	return input
}
