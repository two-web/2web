package reactiveEvent

import (
	"fmt"
	lexer "hudson-newey/2web/src/compiler/2-lexer"
	"hudson-newey/2web/src/models"
	"strings"
)

func FromNode(node lexer.LexNode[lexer.EventNode]) (models.ReactiveEvent, error) {
	if len(node.Tokens) < 4 || node.Tokens[2] != "=" {
		errorMessage := fmt.Errorf("incorrect reactive event assignment:\n\tExpected: @eventName=\"$variable = value\"\n\tFound: %s", node.Selector)
		return models.ReactiveEvent{}, errorMessage
	}

	eventName := node.Tokens[0]
	variableName := node.Tokens[1]
	reducer := strings.Join(node.Tokens[3:], "")

	eventModel := models.ReactiveEvent{
		Node:      &node,
		EventName: eventName,
		VarName:   variableName,
		Reducer:   reducer,
	}

	return eventModel, nil
}
