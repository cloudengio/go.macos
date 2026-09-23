// Usage of docker-entrypoint
//
//	utility to run docker commands with secrets piped into the container and read
//	by the entrypoint command. A container should have 'docker-entrypoint entrypoint'
//	as its entrypoint and the container can be run with 'docker-entrypoint run <docker
//	run flags>...' When run on macos the keychain-item flag can be used to specify
//	a keychain item containing keys in cloudeng.io/cmdutil/keys format that will be
//	piped into the container. The entrypoint command will read the keys from the
//	pipe and write them to the keyring. If the keychain item contains a key with id
//	'my-key' and value 'my-value' then the entrypoint command will write a key to
//	the linux session keyring named 'my-key' with value 'my-value'.
//
//	                   run - run a command with secrets piped into the container. Note run will automatically add 'run -i -t --security-opt seccomp=<profile>' to the docker run command line. Where profile is set to a temp file containing a seccomp profile that allows access to the linux kernel key ring. This profile is created by the 'create-seccomp-profile' command.
//	            entrypoint - entrypoint command to run inside a container
//	create-seccomp-profile - create a seccomp profile that allows access to the linux kernel key ring
package main
