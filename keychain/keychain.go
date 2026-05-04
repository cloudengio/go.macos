// Copyright 2024 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

package keychain

<<<<<<< New base: keychain: keychain sundry updates, docker tests will fail for now.
import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

func (t Type) MarshalYAML() (any, error) {
	return t.String(), nil
}

func (t Type) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t Type) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.String() + `"`), nil
}
||||||| Common ancestor
// Type represents the type of keychain to use.
type Type int
=======
import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func (t Type) MarshalYAML() (any, error) {
	return t.String(), nil
}

func (t Type) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t Type) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.String() + `"`), nil
}
>>>>>>> Current commit: keychain: keychain sundry updates, docker tests will fail for now.

// SecureNoteReader defines the interface for reading secure notes from the keychain.
type SecureNoteReader interface {
	ReadSecureNote(service string) (data []byte, err error)
}
<<<<<<< New base: keychain: keychain sundry updates, docker tests will fail for now.

// Type represents the type of keychain to use.
type Type int

func (t *Type) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return fmt.Errorf("failed to decode keychain type: %w", err)
	}
	kt, err := ParseType(s)
	if err != nil {
		return err
	}
	*t = kt
	return nil
}

func (t *Type) UnmarshalText(text []byte) error {
	kt, err := ParseType(string(text))
	if err != nil {
		return err
	}
	*t = kt
	return nil
}

func (t *Type) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("failed to unmarshal keychain type from JSON: %w", err)
	}
	kt, err := ParseType(s)
	if err != nil {
		return err
	}
	*t = kt
	return nil
}
||||||| Common ancestor
=======

// Type represents the type of keychain to use.
type Type int

func (t *Type) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return fmt.Errorf("failed to decode keychain type: %w", err)
	}
	kt, err := ParseType(s)
	if err != nil {
		return err
	}
	*t = Type(kt)
	return nil
}

func (t *Type) UnmarshalText(text []byte) error {
	kt, err := ParseType(string(text))
	if err != nil {
		return err
	}
	*t = Type(kt)
	return nil
}

func (t *Type) UnmarshalJSON(data []byte) error {
	var s string
	if err := yaml.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("failed to unmarshal keychain type from JSON: %w", err)
	}
	kt, err := ParseType(s)
	if err != nil {
		return err
	}
	*t = Type(kt)
	return nil
}
>>>>>>> Current commit: keychain: keychain sundry updates, docker tests will fail for now.
