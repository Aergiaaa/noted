package service

// linkScope marks which link fields a validation pass must prove point at
// a live row. Create marks every field it carries; update marks only the
// fields the patch moves off the stored value — re-sending the stored id
// is a keep (API.md PATCH: absent/null keeps, "" clears), and the stored
// peer may sit in the trash until purge (DOMAIN.md), so it must not
// re-fail as "unknown" on an unrelated edit.
type linkScope struct {
	task  bool
	event bool
	note  bool
}

// changedLink: the patch presents this field and moves it off the stored
// value, so it is a new link that must exist.
func changedLink(presented *string, stored *string) bool {
	return presented != nil && !sameLink(presented, stored)
}

// sameLink: both values name the same target once nil and "" collapse to
// the same "no value".
func sameLink(a, b *string) bool {
	return derefString(a) == derefString(b)
}
