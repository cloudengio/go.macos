# [cloudeng.io/macos/cmd/docker-entrypoint](https://pkg.go.dev/cloudeng.io/macos/cmd/docker-entrypoint?tab=doc)


panic: field Binary: failed to parse tag: keychain-plugin,,direct path to
the plugin binary, leave empty to use the default

goroutine 1 [running]:
cloudeng.io/cmdutil/subcmd.(*CurrentCommand).MustRunner(0x10d8ecae9440?,
0x10d8ec993e40?, {0x102dc6ed8?, 0x10d8eca803c0?})

    /Users/cnicolaou/LocalOnly/dev/github.com/cloudengio/go.pkgs/cmdutil/subcmd/yaml.go:146 +0x80

main.cli()

    /Users/cnicolaou/LocalOnly/dev/github.com/cloudengio/go.macos/cmd/docker-entrypoint/docker_ep_main.go:48 +0x98

main.main()

    /Users/cnicolaou/LocalOnly/dev/github.com/cloudengio/go.macos/cmd/docker-entrypoint/docker_ep_main.go:56 +0x1c


