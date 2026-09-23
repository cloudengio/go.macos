# [cloudeng.io/macos/cmd/cloudeng-keychain](https://pkg.go.dev/cloudeng.io/macos/cmd/cloudeng-keychain?tab=doc)


panic: field Binary: failed to parse tag: keychain-plugin,,direct path to
the plugin binary, leave empty to use the default

goroutine 1 [running]:
cloudeng.io/cmdutil/subcmd.(*CurrentCommand).MustRunner(0x30a32f663600?,
0x30a32f585d28?, {0x100a38190?, 0x30a32f61a480?})

    /Users/cnicolaou/LocalOnly/dev/github.com/cloudengio/go.pkgs/cmdutil/subcmd/yaml.go:146 +0x80

main.cli.func1(0x30a32f663600)

    /Users/cnicolaou/LocalOnly/dev/github.com/cloudengio/go.macos/cmd/cloudeng-keychain/keychain_cmd.go:62 +0x124

cloudeng.io/cmdutil/subcmd.(*extension).Set(0x30a32f63c880?,
0x30a32f63c880?)

    /Users/cnicolaou/LocalOnly/dev/github.com/cloudengio/go.pkgs/cmdutil/subcmd/extensions.go:61 +0x28

cloudeng.io/cmdutil/subcmd.(*CommandSetYAML).AddExtensions(...)

    /Users/cnicolaou/LocalOnly/dev/github.com/cloudengio/go.pkgs/cmdutil/subcmd/extensions.go:77

main.cli()

    /Users/cnicolaou/LocalOnly/dev/github.com/cloudengio/go.macos/cmd/cloudeng-keychain/keychain_cmd.go:75 +0x29c

main.main()

    /Users/cnicolaou/LocalOnly/dev/github.com/cloudengio/go.macos/cmd/cloudeng-keychain/keychain_cmd.go:85 +0x1c


