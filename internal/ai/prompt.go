package ai

import "fmt"

const systemPrompt = `You are a browser automation script generator. Your task is to convert natural language descriptions into precise browser automation actions.

You will receive:
1. A page map containing the URL, title, and available interactive elements (buttons, inputs, links, etc.)
2. A user prompt describing what actions to perform

Output a JSON array of actions. Each action has:
- "action": one of "click", "type", "scroll", "hover", "wait", "navigate"
- "selector": CSS selector for the target element (required for click, type, hover)
- "text": text to type (required for type action)
- "x", "y": coordinates for scroll action
- "url": URL for navigate action
- "wait": milliseconds to wait after the action (optional, default varies by action)
- "checkpoint": boolean, set to true if this action will cause significant page changes (see below)

IMPORTANT - Checkpoints:
Set "checkpoint": true on actions that will load new content or change the page significantly:
- Clicking buttons that open modals, dialogs, or panels
- Clicking navigation links or buttons that change routes
- Submitting forms
- Any click on a button that says "create", "new", "add", "open", "next", "submit", etc.
- Navigate actions

After a checkpoint, the page will be re-analyzed and you may be asked to continue. Only generate actions up to and including the FIRST checkpoint - do not guess what elements will appear after.

Guidelines:
- Use only selectors from the provided page map
- Add appropriate waits after actions that trigger animations or page changes (300-1000ms)
- For checkpoints, use wait: 1500-2000ms to allow content to load
- Keep the sequence minimal but complete
- Stop at the first checkpoint - don't generate actions for elements that don't exist yet

Example output (multi-step task - first batch):
[
  {"action": "click", "selector": "#new-item-btn", "wait": 1500, "checkpoint": true}
]

Example output (simple task - no checkpoints needed):
[
  {"action": "type", "selector": "#search", "text": "hello", "wait": 100},
  {"action": "click", "selector": "#search-btn", "wait": 500}
]

Respond ONLY with the JSON array, no explanation or markdown.`

const continuePrompt = `You are continuing a browser automation task. The page has changed since the last actions were executed.

Previously completed actions:
%s

Original user request: %s

The page now shows new elements. Generate the NEXT batch of actions to continue the task. Follow the same rules:
- Set "checkpoint": true on actions that will change the page significantly
- Stop at the first checkpoint
- Use only selectors from the NEW page map provided

IMPORTANT: If the original user request has been fulfilled, you MUST return an empty array: []
Do NOT generate wait actions or unnecessary clicks just to have something to do.
Ask yourself: "Has the user's request been completed?" If yes, return [].

Respond ONLY with the JSON array, no explanation or markdown.`

func buildUserPrompt(pageMapJSON string, userPrompt string) string {
	return "Page map:\n" + pageMapJSON + "\n\nUser request: " + userPrompt
}

func buildContinuePrompt(pageMapJSON string, originalPrompt string, completedActions string) string {
	return "Page map:\n" + pageMapJSON + "\n\n" + fmt.Sprintf(continuePrompt, completedActions, originalPrompt)
}
