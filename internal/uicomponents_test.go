package internal

import "testing"

func TestExtractReactComponents(t *testing.T) {
	t.Parallel()

	src := []byte(`type UserCardProps = {
  id: string
  name?: string
}

export function UserCard(props: UserCardProps) {
  return <section>{props.id}</section>
}
`)

	got := extractReactComponents(src, "UserCard.tsx")
	if len(got) != 1 {
		t.Fatalf("len(extractReactComponents) = %d, want 1", len(got))
	}
	if got[0].Framework != UIFrameworkReact {
		t.Fatalf("Framework = %q, want %q", got[0].Framework, UIFrameworkReact)
	}
	if len(got[0].Props) != 2 || got[0].Props[0] != "id" || got[0].Props[1] != "name" {
		t.Fatalf("unexpected props %#v", got[0].Props)
	}
}

func TestExtractVueComponents(t *testing.T) {
	t.Parallel()

	files := []FileInfo{{
		Path:     "components/UserCard.vue",
		Language: "vue",
		Symbols: []Symbol{{
			Name: "defineProps",
			Children: []Symbol{
				{Name: "title"},
				{Name: "subtitle"},
			},
		}},
	}}

	got := extractVueComponents(files)
	if len(got) != 1 {
		t.Fatalf("len(extractVueComponents) = %d, want 1", len(got))
	}
	if got[0].Name != "UserCard" || got[0].Framework != UIFrameworkVue {
		t.Fatalf("unexpected component %#v", got[0])
	}
	if len(got[0].Props) != 2 {
		t.Fatalf("expected 2 props, got %#v", got[0].Props)
	}
}
