package huly

import "testing"

func TestNewCreateIssueTxFields(t *testing.T) {
	attrs := map[string]any{"title": "hello", "priority": int(High)}
	tx := NewCreateIssueTx("proj-1", attrs, "acc-1", 1700000000000)

	if tx["_class"] != TxCreateDoc {
		t.Fatalf("_class = %v", tx["_class"])
	}
	if tx["space"] != SpaceTx {
		t.Fatalf("tx space = %v want %v", tx["space"], SpaceTx)
	}
	if tx["objectClass"] != ClassIssue {
		t.Fatalf("objectClass = %v", tx["objectClass"])
	}
	if tx["objectSpace"] != "proj-1" {
		t.Fatalf("objectSpace = %v", tx["objectSpace"])
	}
	if tx["modifiedBy"] != "acc-1" || tx["createdBy"] != "acc-1" {
		t.Fatalf("modifiedBy/createdBy = %v/%v", tx["modifiedBy"], tx["createdBy"])
	}
	if tx["modifiedOn"] != int64(1700000000000) {
		t.Fatalf("modifiedOn = %v", tx["modifiedOn"])
	}
	a, ok := tx["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes wrong type: %T", tx["attributes"])
	}
	if a["attachedTo"] != IDNoParent {
		t.Fatalf("attachedTo = %v want %v", a["attachedTo"], IDNoParent)
	}
	if a["attachedToClass"] != ClassIssue {
		t.Fatalf("attachedToClass = %v", a["attachedToClass"])
	}
	if a["collection"] != CollectionSubIssues {
		t.Fatalf("collection = %v", a["collection"])
	}
	if a["title"] != "hello" {
		t.Fatalf("title not preserved: %v", a["title"])
	}
	if tx["objectId"] == "" || tx["_id"] == "" {
		t.Fatal("ids must be set")
	}
}

func TestNewUpdateDocTxOps(t *testing.T) {
	tx := NewUpdateDocTx(ClassIssue, "proj-1", "iss-9",
		map[string]any{"title": "new"}, "acc-1", 1700000000000)
	if tx["_class"] != TxUpdateDoc {
		t.Fatalf("_class = %v", tx["_class"])
	}
	if tx["objectId"] != "iss-9" {
		t.Fatalf("objectId = %v", tx["objectId"])
	}
	ops, _ := tx["operations"].(map[string]any)
	if ops["title"] != "new" {
		t.Fatalf("operations.title = %v", ops["title"])
	}
}
