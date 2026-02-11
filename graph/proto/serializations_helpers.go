package proto

import (
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/pquads"
)

// GetNativeValue returns the value stored in Node.
func (m *NodeData) GetNativeValue() quad.Value {
	if m == nil {
		return nil
	} else if m.Value == nil {
		if m.Name == "" {
			return nil
		}
		return quad.Raw(m.Name)
	}
	return m.Value.ToNative()
}

func (m *NodeData) Upgrade() {
	if m.Value == nil {
		m.Value = pquads.MakeValue(quad.Raw(m.Name))
		m.Name = ""
	}
}
