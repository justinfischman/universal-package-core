# Shell completion

`upc completion <shell>` prints a completion script to stdout for `bash`,
`zsh`, `fish` or `powershell`. It needs no project and changes nothing.

## bash

Requires the `bash-completion` package (v2).

```sh
# current session
source <(upc completion bash)
# permanent (Linux)
upc completion bash | sudo tee /etc/bash_completion.d/upc >/dev/null
```

macOS with Homebrew: `brew install bash-completion@2`, then
`upc completion bash > "$(brew --prefix)/etc/bash_completion.d/upc"` and make
sure your `~/.bash_profile` sources
`$(brew --prefix)/etc/profile.d/bash_completion.sh`.

## zsh

```sh
# current session
source <(upc completion zsh)
# permanent: write into a directory on $fpath, then restart the shell
upc completion zsh > "${fpath[1]}/_upc"
```

If completion is not yet enabled, add `autoload -U compinit; compinit` to
`~/.zshrc`. macOS with Homebrew: use
`upc completion zsh > "$(brew --prefix)/share/zsh/site-functions/_upc"`.

## fish

```sh
upc completion fish | source                                  # current session
upc completion fish > ~/.config/fish/completions/upc.fish     # permanent
```

## PowerShell

```powershell
upc completion powershell | Out-String | Invoke-Expression    # current session
upc completion powershell > upc.ps1                           # then dot-source
# it from your $PROFILE for every session
```

Disable the descriptions with `--no-descriptions` (for example
`upc completion bash --no-descriptions`).
