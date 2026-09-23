# [cloudeng.io/macos/cmd/cloudeng-aws-secrets](https://pkg.go.dev/cloudeng.io/macos/cmd/cloudeng-aws-secrets?tab=doc)


panic: field Binary: failed to parse tag: keychain-plugin,,direct path to
the plugin binary, leave empty to use the default

goroutine 1 [running]:
cloudeng.io/cmdutil/subcmd.(*CurrentCommand).MustRunner(0x7fa801e22b00?,
0x7fa801c4de40?, {0x105526900?, 0x7fa801de4820?})

    /Users/cnicolaou/LocalOnly/dev/github.com/cloudengio/go.pkgs/cmdutil/subcmd/yaml.go:146 +0x80

main.cli()

    /Users/cnicolaou/LocalOnly/dev/github.com/cloudengio/go.macos/cmd/cloudeng-aws-secrets/secrets_main.go:43 +0x98

main.main()

    /Users/cnicolaou/LocalOnly/dev/github.com/cloudengio/go.macos/cmd/cloudeng-aws-secrets/secrets_main.go:50 +0x1c


