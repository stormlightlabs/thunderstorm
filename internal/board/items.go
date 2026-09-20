package board

import (
	"context"
	"fmt"
	"sort"
)

// Item is one issue as the board holds it. The status is the board's; the
// title, state and assignees come off the issue in the same read, because
// deciding whether an issue is claimable needs both.
type Item struct {
	// ID is the project item, which is what a field write names. It is not
	// the issue's ID and not its number.
	ID         string   `json:"item"`
	Number     int      `json:"number"`
	Title      string   `json:"title"`
	URL        string   `json:"url"`
	Repository string   `json:"repository"`
	State      string   `json:"state"`
	Status     Status   `json:"status"`
	Option     string   `json:"option"`
	Assignees  []string `json:"assignees"`
	Group      string   `json:"group,omitempty"`

	// IssueID is the issue's node ID, which sub-issue and dependency writes
	// take where the path takes a number.
	IssueID string `json:"issueId"`
}

const itemsGraphQL = `
query($project: ID!, $cursor: String) {
  node(id: $project) {
    ... on ProjectV2 {
      items(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
        nodes {
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
    }
  }
}`

type itemsQuery struct {
	Node struct {
		Items struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []struct {
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
			} `json:"nodes"`
		} `json:"items"`
	} `json:"node"`
}

// pages caps how far a read walks. A project holding more than this has
// outgrown one repository's loop, and walking forever is worse than saying so.
const pages = 20

// Items returns this repository's issues on the board, lowest number first.
// A board carrying several repositories is filtered by both the repository and
// the configured group value, because a run claims only what belongs to it.
func (c *Client) Items(ctx context.Context) ([]Item, error) {
	project, err := c.Project(ctx)
	if err != nil {
		return nil, err
	}

	var (
		items  []Item
		cursor *string
	)
	for page := 0; page < pages; page++ {
		var reply itemsQuery
		variables := map[string]any{"project": project.ID, "cursor": cursor}
		if err := c.query(ctx, itemsGraphQL, variables, &reply); err != nil {
			return nil, err
		}

		for _, node := range reply.Node.Items.Nodes {
			content := node.Content
			// A draft item or a pull request on the board has no issue
			// number, and nothing here can claim or transition one.
			if content.Number == 0 {
				continue
			}
			if content.Repository.NameWithOwner != c.Repository() {
				continue
			}

			item := Item{
				ID:         node.ID,
				Number:     content.Number,
				Title:      content.Title,
				URL:        content.URL,
				Repository: content.Repository.NameWithOwner,
				State:      content.State,
				IssueID:    content.ID,
			}
			item.Assignees = make([]string, 0, len(content.Assignees.Nodes))
			for _, login := range content.Assignees.Nodes {
				item.Assignees = append(item.Assignees, login.Login)
			}
			for _, value := range node.FieldValues.Nodes {
				text := value.Name
				if text == "" {
					text = value.Text
				}
				switch value.Field.Name {
				case c.settings.StatusField:
					item.Option = text
				case c.settings.GroupField:
					item.Group = text
				}
			}
			if c.settings.GroupField != "" && item.Group != c.settings.GroupValue {
				continue
			}
			item.Status, _ = c.status(item.Option)
			items = append(items, item)
		}

		if !reply.Node.Items.PageInfo.HasNextPage {
			break
		}
		next := reply.Node.Items.PageInfo.EndCursor
		cursor = &next
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Number < items[j].Number })
	return items, nil
}

// Queued returns the items in one state.
func (c *Client) Queued(ctx context.Context, status Status) ([]Item, error) {
	all, err := c.Items(ctx)
	if err != nil {
		return nil, err
	}
	var matching []Item
	for _, item := range all {
		if item.Status == status {
			matching = append(matching, item)
		}
	}
	return matching, nil
}

// Find returns one issue's item. An issue that is not on the board is an
// error: its status is not Todo, it is nothing, and the reads here cannot see
// it at all.
func (c *Client) Find(ctx context.Context, number int) (Item, error) {
	items, err := c.Items(ctx)
	if err != nil {
		return Item{}, err
	}
	for _, item := range items {
		if item.Number == number {
			return item, nil
		}
	}
	where := ""
	if c.settings.GroupField != "" {
		where = fmt.Sprintf(" with %s %q", c.settings.GroupField, c.settings.GroupValue)
	}
	return Item{}, fmt.Errorf("issue %s#%d is not on project %d%s: add it with tstorm board add %d",
		c.Repository(), number, c.settings.Number, where, number)
}
