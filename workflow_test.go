package thunderstorm_test

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/stormlightlabs/thunderstorm"
	"github.com/stormlightlabs/thunderstorm/internal/render"
)

// An installed binary renders from what it carries and nothing else, so a file
// the embed misses is a payload that installs short by one skill.
func TestEveryTargetRendersFromTheEmbeddedWorkflow(t *testing.T) {
	src := render.Embedded(thunderstorm.Workflow())
	m, err := render.Load(src)
	if err != nil {
		t.Fatalf("load the embedded manifest: %v", err)
	}
	for _, target := range render.Targets() {
		t.Run(target.Name, func(t *testing.T) {
			payload, err := render.Plan(m, target, src)
			if err != nil {
				// A target that carries nothing says so through *Unmet, which
				// is an answer rather than a broken embed.
				var unmet *render.Unmet
				if errors.As(err, &unmet) {
					t.Skipf("%s carries no payload: %v", target.Name, err)
				}
				t.Fatalf("plan: %v", err)
			}
			if len(payload.Files) == 0 {
				t.Fatal("the payload holds no files")
			}
		})
	}
}

// The embedded tree is what every install reads, so the files the manifest
// does not list still have to be there for the ones it does.
func TestTheEmbeddedWorkflowCarriesASkillsReferences(t *testing.T) {
	body, err := fs.ReadFile(thunderstorm.Workflow(), "skills/writing-docs/references/tells.md")
	if err != nil {
		t.Fatalf("read a skill reference: %v", err)
	}
	if len(body) == 0 {
		t.Error("the catalogue is empty")
	}
}
