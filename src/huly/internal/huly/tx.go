package huly

// Tx is a generic Huly transaction envelope.
type Tx map[string]any

func baseTx(class, modifiedBy string, now int64) Tx {
	id := NewRef()
	return Tx{
		"_id":        id,
		"_class":     class,
		"space":      SpaceTx,
		"modifiedOn": now,
		"modifiedBy": modifiedBy,
		"createdOn":  now,
		"createdBy":  modifiedBy,
	}
}

// NewCreateDocTx builds a TxCreateDoc for a plain Doc (Milestone, Component, ...).
func NewCreateDocTx(objectClass, objectSpace string, attrs map[string]any, modifiedBy string, now int64) Tx {
	if attrs == nil {
		attrs = map[string]any{}
	}
	tx := baseTx(TxCreateDoc, modifiedBy, now)
	tx["objectId"] = NewRef()
	tx["objectClass"] = objectClass
	tx["objectSpace"] = objectSpace
	tx["attributes"] = attrs
	return tx
}

// NewCreateIssueTx builds a TxCreateDoc for an Issue (an AttachedDoc). It injects
// the collection-membership fields, defaulting the issue to top-level (NoParent).
func NewCreateIssueTx(space string, attrs map[string]any, modifiedBy string, now int64) Tx {
	if attrs == nil {
		attrs = map[string]any{}
	}
	if _, ok := attrs["attachedTo"]; !ok {
		attrs["attachedTo"] = IDNoParent
	}
	attrs["attachedToClass"] = ClassIssue
	attrs["collection"] = CollectionSubIssues
	return NewCreateDocTx(ClassIssue, space, attrs, modifiedBy, now)
}

// NewUpdateDocTx builds a TxUpdateDoc.
func NewUpdateDocTx(objectClass, objectSpace, objectID string, ops map[string]any, modifiedBy string, now int64) Tx {
	tx := baseTx(TxUpdateDoc, modifiedBy, now)
	tx["objectId"] = objectID
	tx["objectClass"] = objectClass
	tx["objectSpace"] = objectSpace
	tx["operations"] = ops
	return tx
}

// NewRemoveDocTx builds a TxRemoveDoc.
func NewRemoveDocTx(objectClass, objectSpace, objectID, modifiedBy string, now int64) Tx {
	tx := baseTx(TxRemoveDoc, modifiedBy, now)
	tx["objectId"] = objectID
	tx["objectClass"] = objectClass
	tx["objectSpace"] = objectSpace
	return tx
}
