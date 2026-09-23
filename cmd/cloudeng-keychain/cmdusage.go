// Usage of cloudeng-keychain
//
//	provide access to local keychains across multiple operating systems
//
//	    read - read an item from the keychain writing to filename, if filename is - the item will be written to stdout. Valid values for flags are as follows:
//	 --keychain-accessibility: after-first-unlock, after-first-unlock-this-device-only, always, always-this-device-only, default, when-passcode-set-this-device-only, when-unlocked, when-unlocked-this-device-only
//	 --keychain-type: all, data-protection-local, file, icloud
//	   write - write an item read from <filename> to the keychain, if filename is - the item will be read from stdin. Valid values for flags are as follows:
//	  --keychain-accessibility: after-first-unlock, after-first-unlock-this-device-only, always, always-this-device-only, default, when-passcode-set-this-device-only, when-unlocked, when-unlocked-this-device-only
//	  --keychain-type: data-protection-local, file, icloud
//	key-info - manage key info items in a keychain/secrets store, multiple key info items can be stored in a single item. In all cases if input or output is a filename, then "-" or "" will result in stdin or stdout being used as appropriate.
//
// global flags: [--verbose=false]
//
//	-verbose
//	  set to enable verbose logging
package main
