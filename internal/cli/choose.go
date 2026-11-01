package cli

import (
	"fmt"
	"sort"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// pick chooses one of options: the only one automatically, the registered
// default under --yes, otherwise by asking. A non-interactive run that cannot
// decide fails naming the flag that would.
func pick(s *Session, what string, options []string, fallback, flag string) (string, error) {
	switch {
	case len(options) == 0:
		return "", fmt.Errorf("%w: no %s available", core.ErrInvalidRequest, what)
	case len(options) == 1:
		return options[0], nil
	}
	if s.Yes {
		for _, o := range options {
			if o == fallback {
				return o, nil
			}
		}
		return options[0], nil
	}
	choice, err := s.Prompter.Choose("Choose a "+what+":", options)
	if err != nil {
		return "", fmt.Errorf("%w (or pass %s)", err, flag)
	}
	return choice, nil
}

// languageFor returns the registered language or a usage error listing the known ones.
func languageFor(regs core.Registries, id string) (core.Language, error) {
	lang, ok := regs.Languages.Get(id)
	if !ok {
		return nil, fmt.Errorf("%w: unknown language %q (known: %v)", core.ErrInvalidRequest, id, regs.Languages.IDs())
	}
	return lang, nil
}

// managerFor returns the registered manager or a usage error listing the known ones.
func managerFor(regs core.Registries, id string) (core.PackageManager, error) {
	pm, ok := regs.PackageManagers.Get(id)
	if !ok {
		return nil, fmt.Errorf("%w: unknown package manager %q (known: %v)", core.ErrInvalidRequest, id, regs.PackageManagers.IDs())
	}
	return pm, nil
}

// choice is an explicit or configured language/manager selection; either may be empty.
type choice struct {
	language string
	manager  string
}

// chooseStack resolves the language and manager for a new project of type pt:
// explicit flags win, then config, then a single candidate, then --yes
// defaults, then a prompt. supports filters the candidates.
func chooseStack(s *Session, want choice, supports func(language, manager string) bool) (core.Language, core.PackageManager, error) {
	regs := s.Registries
	if want.manager != "" {
		pm, err := managerFor(regs, want.manager)
		if err != nil {
			return nil, nil, err
		}
		if want.language != "" && want.language != pm.Language() {
			return nil, nil, fmt.Errorf("%w: package manager %q belongs to language %q, not %q",
				core.ErrIncompatible, pm.ID(), pm.Language(), want.language)
		}
		lang, err := languageFor(regs, pm.Language())
		return lang, pm, err
	}

	var langID string
	if want.language != "" {
		langID = want.language
	} else {
		var options []string
		for _, l := range regs.Languages.List() {
			if supports(l.ID(), "") {
				options = append(options, l.ID())
			}
		}
		var err error
		if langID, err = pick(s, "language", options, regs.DefaultLanguage, "--language"); err != nil {
			return nil, nil, err
		}
	}
	lang, err := languageFor(regs, langID)
	if err != nil {
		return nil, nil, err
	}

	var managers []string
	for _, id := range lang.PackageManagers() {
		if supports(lang.ID(), id) {
			managers = append(managers, id)
		}
	}
	sort.Strings(managers)
	id, err := pick(s, "package manager", managers, lang.DefaultPackageManager(), "--pm")
	if err != nil {
		return nil, nil, err
	}
	pm, err := managerFor(regs, id)
	return lang, pm, err
}
