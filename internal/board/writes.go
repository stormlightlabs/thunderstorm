package board

import (
	"context"
	"fmt"
)

const setFieldGraphQL = `
mutation($project: ID!, $item: ID!, $field: ID!, $option: String!) {
  updateProjectV2ItemFieldValue(input: {
    projectId: $project, itemId: $item, fieldId: $field,
    value: { singleSelectOptionId: $option }
  }) { projectV2Item { id } }
}`

const setTextGraphQL = `
mutation($project: ID!, $item: ID!, $field: ID!, $text: String!) {
  updateProjectV2ItemFieldValue(input: {
    projectId: $project, itemId: $item, fieldId: $field,
    value: { text: $text }
  }) { projectV2Item { id } }
}`

const addItemGraphQL = `
mutation($project: ID!, $content: ID!) {
  addProjectV2ItemById(input: { projectId: $project, contentId: $content }) {
    item { id }
  }
}`

const itemGraphQL = `
query($item: ID!) {
  node(id: $item) {
    ... on ProjectV2Item {
      id
      fieldValues(first: 20) {
        nodes {
          ... on ProjectV2ItemFieldSingleSelectValue {
            name
            field { ... on ProjectV2FieldCommon { name } }
          }
          ... on ProjectV2ItemFieldTextValue {
            text
            field { ... on ProjectV2FieldCommon { name } }
          }
        }
      }
      content {
        ... on Issue {
          id
          number
          title
          url
          state
          repository { nameWithOwner }
          assignees(first: 20) { nodes { login } }
        }
      }
    }
  }
}`

// SetStatus writes one status option onto one item. The write carries no
// condition, which is why every caller reads the item back afterwards rather
// than trusting the mutation's own answer.
func (c *Client) SetStatus(ctx context.Context, item string, status Status) error {
	project, err := c.Project(ctx)
	if err != nil {
		return err
	}
	option, ok := project.Status.Options[c.OptionName(status)]
	if !ok {
		return fmt.Errorf("field %q has no option %q", project.Status.Name, c.OptionName(status))
	}
	return c.query(ctx, setFieldGraphQL, map[string]any{
		"project": project.ID,
		"item":    item,
		"field":   project.Status.ID,
		"option":  option,
	}, nil)
}

// setGroup writes the configured group value onto a new item, so a board
// carrying several repositories' work can tell which one filed it. An item
// with no group value is invisible to every read here.
func (c *Client) setGroup(ctx context.Context, item string) error {
	project, err := c.Project(ctx)
	if err != nil {
		return err
	}
	if project.Group.ID == "" {
		return nil
	}
	variables := map[string]any{"project": project.ID, "item": item, "field": project.Group.ID}
	if !project.Group.singleSelect() {
		variables["text"] = c.settings.GroupValue
		return c.query(ctx, setTextGraphQL, variables, nil)
	}
	option, ok := project.Group.Options[c.settings.GroupValue]
	if !ok {
		return fmt.Errorf("field %q has no option %q, so a new issue would be filed where no read finds it",
			project.Group.Name, c.settings.GroupValue)
	}
	variables["option"] = option
	return c.query(ctx, setFieldGraphQL, variables, nil)
}

// Add puts an issue on the board and gives it the loop's starting state.
// An issue that never reaches the board is invisible to every read here.
func (c *Client) Add(ctx context.Context, issue Issue) (Item, error) {
	project, err := c.Project(ctx)
	if err != nil {
		return Item{}, err
	}

	var reply struct {
		AddProjectV2ItemByID struct {
			Item struct {
				ID string `json:"id"`
			} `json:"item"`
		} `json:"addProjectV2ItemById"`
	}
	err = c.query(ctx, addItemGraphQL, map[string]any{
		"project": project.ID,
		"content": issue.NodeID,
	}, &reply)
	if err != nil {
		return Item{}, err
	}

	item := reply.AddProjectV2ItemByID.Item.ID
	if err := c.setGroup(ctx, item); err != nil {
		return Item{}, err
	}
	if err := c.SetStatus(ctx, item, Todo); err != nil {
		return Item{}, err
	}
	return c.refresh(ctx, item)
}

// refresh reads one item back. Every write here is followed by one: the
// mutation reports what it was asked to do, and the board is what says what
// happened.
func (c *Client) refresh(ctx context.Context, item string) (Item, error) {
	var reply struct {
		Node struct {
			ID          string `json:"id"`
			FieldValues struct {
				Nodes []struct {
					Name  string `json:"name"`
					Text  string `json:"text"`
					Field struct {
						Name string `json:"name"`
					} `json:"field"`
				} `json:"nodes"`
			} `json:"fieldValues"`
			Content struct {
				ID         string `json:"id"`
				Number     int    `json:"number"`
				Title      string `json:"title"`
				URL        string `json:"url"`
				State      string `json:"state"`
				Repository struct {
					NameWithOwner string `json:"nameWithOwner"`
				} `json:"repository"`
				Assignees struct {
					Nodes []struct {
						Login string `json:"login"`
					} `json:"nodes"`
				} `json:"assignees"`
			} `json:"content"`
		} `json:"node"`
	}
	if err := c.query(ctx, itemGraphQL, map[string]any{"item": item}, &reply); err != nil {
		return Item{}, err
	}

	node := reply.Node
	read := Item{
		ID:         node.ID,
		Number:     node.Content.Number,
		Title:      node.Content.Title,
		URL:        node.Content.URL,
		Repository: node.Content.Repository.NameWithOwner,
		State:      node.Content.State,
		IssueID:    node.Content.ID,
	}
	read.Assignees = make([]string, 0, len(node.Content.Assignees.Nodes))
	for _, login := range node.Content.Assignees.Nodes {
		read.Assignees = append(read.Assignees, login.Login)
	}
	for _, value := range node.FieldValues.Nodes {
		text := value.Name
		if text == "" {
			text = value.Text
		}
		switch value.Field.Name {
		case c.settings.StatusField:
			read.Option = text
		case c.settings.GroupField:
			read.Group = text
		}
	}
	read.Status, _ = c.status(read.Option)
	return read, nil
}
