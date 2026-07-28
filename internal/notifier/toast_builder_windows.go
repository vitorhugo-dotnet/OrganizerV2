//go:build windows

package notifier

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
	"git.sr.ht/~jackmordaunt/go-toast/v2/wintoast"
	"github.com/vitorhugo-java/organizerv2/internal/config"
)

type windowsToast struct {
	toast.Notification
	DefaultInput string
}

func buildWindowsToast(event notificationEvent, cfg config.NotificationConfig, shortcuts []resolvedShortcut, activationExe string) windowsToast {
	currentDirectory := filepath.Clean(filepath.Dir(event.CurrentPath))
	selections := []toast.InputSelection{
		{ID: currentDestinationSelectionID, Content: currentDestinationLabel(currentDirectory)},
	}
	if cfg.Actions.MoveTo {
		for _, shortcut := range shortcuts {
			if sameDestinationDirectory(event.CurrentPath, shortcut.Path) {
				continue
			}
			selections = append(selections, toast.InputSelection{
				ID:      shortcut.ID,
				Content: shortcut.Name,
			})
		}
	}

	notification := windowsToast{
		Notification: toast.Notification{
			AppID:         windowsToastAppID,
			Title:         fmt.Sprintf("File %s moved to %s.", filepath.Base(event.CurrentPath), event.Category),
			Body:          "Organizer",
			ActivationExe: activationExe,
			Inputs: []toast.Input{
				{
					ID:         destinationInputID,
					Title:      "Move file to",
					Selections: selections,
				},
			},
		},
		DefaultInput: currentDestinationSelectionID,
	}

	appendAction := func(enabled bool, content string, action notificationAction, inputID string) {
		if !enabled {
			return
		}
		notification.Actions = append(notification.Actions, toast.Action{
			Type:      toast.Foreground,
			Content:   content,
			Arguments: encodeNotificationAction(action, event.ID),
			InputID:   inputID,
		})
	}

	appendAction(cfg.Actions.OpenLocation, "Open Location", actionOpenLocation, destinationInputID)
	appendAction(cfg.Actions.OpenFile, "Open File", actionOpenFile, destinationInputID)
	appendAction(cfg.Actions.Confirm, "Confirm", actionConfirm, "")

	return notification
}

func currentDestinationLabel(directory string) string {
	name := filepath.Base(filepath.Clean(directory))
	if name == "" || name == "." {
		name = filepath.Clean(directory)
	}
	return name + " (current)"
}

func (n windowsToast) XML() (string, error) {
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)

	rootAttributes := make([]xml.Attr, 0, 2)
	if n.ActivationType != "" {
		rootAttributes = append(rootAttributes, xmlAttribute("activationType", n.ActivationType))
	}
	if n.ActivationArguments != "" {
		rootAttributes = append(rootAttributes, xmlAttribute("launch", n.ActivationArguments))
	}
	if err := encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "toast"}, Attr: rootAttributes}); err != nil {
		return "", fmt.Errorf("start toast XML: %w", err)
	}

	if err := encodeToastVisual(encoder, n.Title, n.Body); err != nil {
		return "", err
	}
	if err := encodeToastActions(encoder, n.Inputs, n.Actions, n.DefaultInput); err != nil {
		return "", err
	}

	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "toast"}}); err != nil {
		return "", fmt.Errorf("end toast XML: %w", err)
	}
	if err := encoder.Flush(); err != nil {
		return "", fmt.Errorf("flush toast XML: %w", err)
	}
	return output.String(), nil
}

func (n windowsToast) Push() error {
	payload, err := n.XML()
	if err != nil {
		return err
	}
	return wintoast.Push(payload)
}

func encodeToastVisual(encoder *xml.Encoder, title, body string) error {
	visual := xml.StartElement{Name: xml.Name{Local: "visual"}}
	binding := xml.StartElement{
		Name: xml.Name{Local: "binding"},
		Attr: []xml.Attr{xmlAttribute("template", "ToastGeneric")},
	}
	if err := encoder.EncodeToken(visual); err != nil {
		return fmt.Errorf("start toast visual: %w", err)
	}
	if err := encoder.EncodeToken(binding); err != nil {
		return fmt.Errorf("start toast binding: %w", err)
	}
	for _, content := range []string{title, body} {
		if content == "" {
			continue
		}
		if err := encoder.EncodeElement(content, xml.StartElement{Name: xml.Name{Local: "text"}}); err != nil {
			return fmt.Errorf("encode toast text: %w", err)
		}
	}
	if err := encoder.EncodeToken(binding.End()); err != nil {
		return fmt.Errorf("end toast binding: %w", err)
	}
	if err := encoder.EncodeToken(visual.End()); err != nil {
		return fmt.Errorf("end toast visual: %w", err)
	}
	return nil
}

func encodeToastActions(encoder *xml.Encoder, inputs []toast.Input, actions []toast.Action, defaultInput string) error {
	if len(inputs) == 0 && len(actions) == 0 {
		return nil
	}
	actionsElement := xml.StartElement{Name: xml.Name{Local: "actions"}}
	if err := encoder.EncodeToken(actionsElement); err != nil {
		return fmt.Errorf("start toast actions: %w", err)
	}

	for _, input := range inputs {
		inputType := "text"
		if len(input.Selections) > 0 {
			inputType = "selection"
		}
		attributes := []xml.Attr{
			xmlAttribute("id", input.ID),
			xmlAttribute("type", inputType),
		}
		if input.Title != "" {
			attributes = append(attributes, xmlAttribute("title", input.Title))
		}
		if input.Placeholder != "" {
			attributes = append(attributes, xmlAttribute("placeHolderContent", input.Placeholder))
		}
		if input.ID == destinationInputID && defaultInput != "" {
			attributes = append(attributes, xmlAttribute("defaultInput", defaultInput))
		}

		inputElement := xml.StartElement{Name: xml.Name{Local: "input"}, Attr: attributes}
		if err := encoder.EncodeToken(inputElement); err != nil {
			return fmt.Errorf("start toast input: %w", err)
		}
		for _, selection := range input.Selections {
			selectionElement := xml.StartElement{
				Name: xml.Name{Local: "selection"},
				Attr: []xml.Attr{
					xmlAttribute("id", selection.ID),
					xmlAttribute("content", selection.Content),
				},
			}
			if err := encoder.EncodeToken(selectionElement); err != nil {
				return fmt.Errorf("start toast selection: %w", err)
			}
			if err := encoder.EncodeToken(selectionElement.End()); err != nil {
				return fmt.Errorf("end toast selection: %w", err)
			}
		}
		if err := encoder.EncodeToken(inputElement.End()); err != nil {
			return fmt.Errorf("end toast input: %w", err)
		}
	}

	for _, action := range actions {
		attributes := []xml.Attr{
			xmlAttribute("activationType", action.Type),
			xmlAttribute("content", action.Content),
			xmlAttribute("arguments", action.Arguments),
		}
		if action.InputID != "" {
			attributes = append(attributes, xmlAttribute("hint-inputId", action.InputID))
		}
		actionElement := xml.StartElement{Name: xml.Name{Local: "action"}, Attr: attributes}
		if err := encoder.EncodeToken(actionElement); err != nil {
			return fmt.Errorf("start toast action: %w", err)
		}
		if err := encoder.EncodeToken(actionElement.End()); err != nil {
			return fmt.Errorf("end toast action: %w", err)
		}
	}

	if err := encoder.EncodeToken(actionsElement.End()); err != nil {
		return fmt.Errorf("end toast actions: %w", err)
	}
	return nil
}

func xmlAttribute(name, value string) xml.Attr {
	return xml.Attr{Name: xml.Name{Local: name}, Value: value}
}
