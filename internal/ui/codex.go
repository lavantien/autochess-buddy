package ui

import (
	"strconv"
	"strings"

	"github.com/lavantien/autochess-buddy/internal/domain"
)

// CodexFormState carries one codex form's raw values and field errors so 422
// rerenders preserve exactly what was typed.
type CodexFormState struct {
	Values map[string]string
	Errs   []domain.FieldError
	// Components holds the recipe multi-select's raw option values.
	Components []string
}

func NewCodexForm() CodexFormState {
	return CodexFormState{Values: map[string]string{}}
}

// entityPlural is the inverse of codexSingular, explicit because english plurals
// are not a suffix rule.
func entityPlural(singular string) string {
	switch singular {
	case "hero":
		return "heroes"
	case "race":
		return "races"
	case "class":
		return "classes"
	case "item":
		return "items"
	case "relic":
		return "relics"
	case "patch":
		return "patches"
	case "pro":
		return "pros"
	}
	return singular
}

func (st CodexFormState) val(key, fallback string) string {
	if v, ok := st.Values[key]; ok && v != "" {
		return v
	}
	return fallback
}

func (st CodexFormState) err(field string) string {
	for _, fe := range st.Errs {
		if fe.Field == field {
			return fe.Msg
		}
	}
	return ""
}

// LadderKey scopes a synergies form field to one race or class so errors and
// typed values land under the ladder that issued them. Handlers build error
// fields and value keys with it; the template binds with the same call.
func LadderKey(entity string, id int64, field string) string {
	return entity + "-" + itoa(id) + "-" + field
}

func (st CodexFormState) firstErrField() string {
	if len(st.Errs) == 0 {
		return ""
	}
	return st.Errs[0].Field
}

// autofocus reports whether this field is the first bad one and should take focus.
func (st CodexFormState) autofocus(field string) bool {
	return st.firstErrField() == field
}

// hasComponent reports whether the recipe picker preselects this item.
func (st CodexFormState) hasComponent(id int64) bool {
	want := strconv.FormatInt(id, 10)
	for _, c := range st.Components {
		if c == want {
			return true
		}
	}
	return false
}

// plainRow is one simple codex table row: label, link, extra cell.
type plainRow struct {
	Label string
	Href  string
	Extra string
}

func relicRows(relics []domain.Relic) []plainRow {
	rows := make([]plainRow, len(relics))
	for i, r := range relics {
		rows[i] = plainRow{Label: r.Name, Href: "/relics/" + itoa(r.ID), Extra: r.Effect}
	}
	return rows
}

func patchRows(patches []domain.Patch) []plainRow {
	rows := make([]plainRow, len(patches))
	for i, p := range patches {
		rows[i] = plainRow{Label: p.Version, Href: "/patches/" + itoa(p.ID), Extra: p.ReleasedAt}
	}
	return rows
}

func proRows(pros []domain.Pro) []plainRow {
	rows := make([]plainRow, len(pros))
	for i, p := range pros {
		rows[i] = plainRow{Label: p.Name, Href: "/pros/" + itoa(p.ID), Extra: p.Handle}
	}
	return rows
}

func lineageNames(races []domain.Race) string {
	parts := make([]string, len(races))
	for i, r := range races {
		parts[i] = r.Name
	}
	return strings.Join(parts, ", ")
}

func lineageNamesC(classes []domain.Class) string {
	parts := make([]string, len(classes))
	for i, c := range classes {
		parts[i] = c.Name
	}
	return strings.Join(parts, ", ")
}

// avgText renders a codex analytics column; blank when the hero has no plays.
func avgText(avg float64) string {
	if avg == 0 {
		return ""
	}
	return strconv.FormatFloat(avg, 'f', 1, 64)
}

func heroAction(id int64) string {
	if id == 0 {
		return "/heroes"
	}
	return "/heroes/" + itoa(id)
}

func itemAction(id int64) string {
	if id == 0 {
		return "/items"
	}
	return "/items/" + itoa(id)
}

func plainAction(entity string, id int64) string {
	if id == 0 {
		return "/" + entity
	}
	return "/" + entity + "/" + itoa(id)
}
