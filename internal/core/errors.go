package core

type ActionError struct {
	Code           string
	MessageKey     string
	MessageData    map[string]any
	SuggestionKey  string
	SuggestionData map[string]any
	Cause          error
}

func NewActionError(code, messageKey string, cause error, suggestionKey string, messageData, suggestionData map[string]any) *ActionError {
	return &ActionError{
		Code:           code,
		MessageKey:     messageKey,
		MessageData:    messageData,
		SuggestionKey:  suggestionKey,
		SuggestionData: suggestionData,
		Cause:          cause,
	}
}

func (e *ActionError) Error() string {
	return e.Code + ": " + e.MessageKey
}

func (e *ActionError) Unwrap() error {
	return e.Cause
}
