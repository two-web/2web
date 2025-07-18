package templating

import (
	"fmt"
	lexer "hudson-newey/2web/src/compiler/2-lexer"
	parser "hudson-newey/2web/src/compiler/4-parser"
	"hudson-newey/2web/src/compiler/5-templating/controlFlow"
	"hudson-newey/2web/src/compiler/5-templating/reactiveCompiler"
	"hudson-newey/2web/src/content/document/documentErrors"
	"hudson-newey/2web/src/content/html"
	"hudson-newey/2web/src/content/page"
	"hudson-newey/2web/src/models"
	"hudson-newey/2web/src/models/reactiveEvent"
	"hudson-newey/2web/src/models/reactiveProperty"
	"hudson-newey/2web/src/models/reactiveVariable"
	"strings"
)

// TODO: Page/component models should have their associated reactive models as
// properties.
func Compile(filePath string, pageModel page.Page, ast []parser.Node) page.Page {
	pageModel.Html.Content = controlFlow.ProcessControlFlow(filePath, pageModel.Html.Content)

	if html.IsHtmlFile(filePath) {
		pageModel.Html.Content = expandElementRefs(pageModel.Html.Content)
	}

	// "Mustache like" expressions e.g. {{ $count }} are a shorthand for an
	// element with only innerText.
	// Therefore, we expand all of the mustache expressions before finding
	// reactive property tokens
	pageModel.Html.Content = expandTextNodes(pageModel.Html.Content)

	importNodes := []lexer.LexNode[lexer.ImportNode]{}
	reactiveVariables := []*models.ReactiveVariable{}

	propertyNodes := lexer.FindPropNodes[lexer.PropNode](pageModel.Html.Content, propertyPrefix)
	eventNodes := lexer.FindPropNodes[lexer.EventNode](pageModel.Html.Content, eventPrefix)

	for _, node := range pageModel.JavaScript {
		// At the moment, we do not support mixing compiler and runtime scripts
		if !node.IsCompilerOnly() {
			continue
		}

		lineCommentNodes := lexer.FindNodes[lexer.LineCommentNode](node.Content, lineCommentStartToken, newLineToken)
		lineCommentRemoved := node.Content
		for _, commentNode := range lineCommentNodes {
			lineCommentRemoved = strings.ReplaceAll(lineCommentRemoved, commentNode.Selector, "")
		}

		// We cannot do this the same time as line comments because someone might
		// decide to put line comments inside of a block comment (or vice versa).
		// Meaning that the selectors would no longer match.
		blockCommentNodes := lexer.FindNodes[lexer.BlockCommentNode](lineCommentRemoved, blockCommentStartToken, blockCommentEndToken)
		noCommentsContent := lineCommentRemoved
		for _, commentNode := range blockCommentNodes {
			noCommentsContent = strings.ReplaceAll(noCommentsContent, commentNode.Selector, "")
		}

		variableNodes := lexer.FindNodes[lexer.VarNode](noCommentsContent, variableToken, statementEndToken)

		for _, variableNode := range variableNodes {
			variableModel, err := reactiveVariable.FromNode(variableNode)
			if err != nil {
				documentErrors.AddErrors(models.Error{
					FilePath: filePath,
					Message:  err.Error(),
				})
				continue
			}

			reactiveVariables = append(reactiveVariables, &variableModel)
		}

		newImportNodes := lexer.FindNodes[lexer.ImportNode](node.Content, importPrefix, statementEndToken)
		importNodes = append(importNodes, newImportNodes...)
	}

	for _, node := range propertyNodes {
		property, err := reactiveProperty.FromNode(node)
		if err != nil {
			documentErrors.AddErrors(models.Error{
				FilePath: filePath,
				Message:  err.Error(),
			})
			continue
		}

		// find the reactive variable that matches the binding name
		foundAssociatedProperty := false
		for _, variable := range reactiveVariables {
			if variable.Name == property.VarName {
				variable.AddProp(&property)
				property.BindVariable(variable)
				foundAssociatedProperty = true
				break
			}
		}

		if !foundAssociatedProperty {
			errorMessage := fmt.Sprintf("could not find compiler variable '%s' for property %s", property.VarName, property.Node.Selector)
			documentErrors.AddErrors(models.Error{
				FilePath: filePath,
				Message:  errorMessage,
			})
			continue
		}
	}

	for _, node := range eventNodes {
		event, err := reactiveEvent.FromNode(node)
		if err != nil {
			documentErrors.AddErrors(models.Error{
				FilePath: filePath,
				Message:  err.Error(),
			})
			continue
		}

		foundAssociatedProperty := false
		for _, variable := range reactiveVariables {
			if variable.Name == event.VarName {
				variable.AddEvent(&event)
				event.BindVariable(variable)
				foundAssociatedProperty = true
				break
			}
		}

		if !foundAssociatedProperty {
			errorMessage := fmt.Sprintf("could not find compiler variable '%s' for event %s", event.VarName, event.Node.Selector)
			documentErrors.AddErrors(models.Error{
				FilePath: filePath,
				Message:  errorMessage,
			})
			continue
		}
	}

	pageModel = evaluateImports(filePath, pageModel, importNodes)

	pageModel = reactiveCompiler.CompileReactivity(filePath, pageModel, reactiveVariables)

	return pageModel
}
