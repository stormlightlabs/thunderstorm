package board

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Project is what one lookup of the board answers: the node IDs every write
// needs, and the option ID behind each status name.
type Project struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Number int    `json:"number"`

	Status Field `json:"status"`
	// Group is the field separating one repository's work from another's on a
	// shared board. A board that carries one repository has no group field and
	// this is zero.
	Group Field `json:"group,omitzero"`
}

// Field is one project field and, for a single select, its options by name.
type Field struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Options map[string]string `json:"-"`
}

// singleSelect reports whether the field was declared with options, which is
// what decides the shape of a write to it.
func (f Field) singleSelect() bool { return len(f.Options) > 0 }

type projectQuery struct {
	RepositoryOwner *struct {
		ProjectV2 *struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Number int    `json:"number"`
			Fields struct {
				Nodes []struct {
					ID      string `json:"id"`
					Name    string `json:"name"`
					Options []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"options"`
				} `json:"nodes"`
			} `json:"fields"`
		} `json:"projectV2"`
	} `json:"repositoryOwner"`
}

const projectGraphQL = `
query($owner: String!, $number: Int!) {
  repositoryOwner(login: $owner) {
    ... on ProjectV2Owner {
      projectV2(number: $number) {
        id
        title
        number
        fields(first: 50) {
          nodes {
            ... on ProjectV2FieldCommon { id name }
            ... on ProjectV2SingleSelectField { id name options { id name } }
          }
        }
      }
    }
  }
}`

// Project looks the board up once per run and keeps the answer. Every
// operation needs the project's node ID and the status options, and neither
// moves while a command runs.
func (c *Client) Project(ctx context.Context) (*Project, error) {
	if c.project != nil {
		return c.project, nil
	}

	var reply projectQuery
	err := c.query(ctx, projectGraphQL, map[string]any{
		"owner":  c.settings.Owner,
		"number": c.settings.Number,
	}, &reply)
	if err != nil {
		return nil, err
	}
	if reply.RepositoryOwner == nil || reply.RepositoryOwner.ProjectV2 == nil {
		return nil, fmt.Errorf("no project %d under %s, or the token cannot read it%s",
			c.settings.Number, c.settings.Owner, scopeHint(403, "scope"))
	}

	found := reply.RepositoryOwner.ProjectV2
	project := &Project{ID: found.ID, Title: found.Title, Number: found.Number}
	var names []string
	for _, node := range found.Fields.Nodes {
		if node.Name == "" {
			continue
		}
		names = append(names, node.Name)
		options := map[string]string{}
		for _, option := range node.Options {
			options[option.Name] = option.ID
		}
		field := Field{ID: node.ID, Name: node.Name, Options: options}
		switch node.Name {
		case c.settings.StatusField:
			project.Status = field
		case c.settings.GroupField:
			project.Group = field
		}
	}

	if project.Status.ID == "" {
		return nil, fmt.Errorf("project %d has no field named %q, only %s",
			found.Number, c.settings.StatusField, strings.Join(sorted(names), ", "))
	}
	if !project.Status.singleSelect() {
		return nil, fmt.Errorf("field %q on project %d is not a single select",
			c.settings.StatusField, found.Number)
	}
	if c.settings.GroupField != "" && project.Group.ID == "" {
		return nil, fmt.Errorf("project %d has no field named %q, only %s",
			found.Number, c.settings.GroupField, strings.Join(sorted(names), ", "))
	}
	for _, status := range Statuses() {
		option := c.OptionName(status)
		if _, ok := project.Status.Options[option]; !ok {
			return nil, fmt.Errorf("field %q on project %d has no option %q, only %s",
				c.settings.StatusField, found.Number, option,
				strings.Join(sorted(keys(project.Status.Options)), ", "))
		}
	}

	c.project = project
	return project, nil
}

// query sends one GraphQL request and decodes data into out. GraphQL answers
// 200 with an errors array, so a caller checking the status alone reads a
// refusal as a result.
func (c *Client) query(ctx context.Context, document string, variables map[string]any, out any) error {
	var reply struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	body := map[string]any{"query": document, "variables": variables}
	if err := c.call(ctx, "POST", c.graphql, body, &reply); err != nil {
		return err
	}
	if len(reply.Errors) > 0 {
		messages := make([]string, 0, len(reply.Errors))
		for _, e := range reply.Errors {
			messages = append(messages, e.Message)
		}
		joined := strings.Join(messages, "; ")
		return fmt.Errorf("graphql: %s%s", joined, scopeHint(200, joined))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(reply.Data, out)
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sorted(values []string) []string {
	sort.Strings(values)
	return values
}
