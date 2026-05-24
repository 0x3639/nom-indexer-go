package dto

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

func TestFromMomentum_Nil(t *testing.T) {
	if got := FromMomentum(nil); got != nil {
		t.Errorf("FromMomentum(nil) = %v, want nil", got)
	}
}

func TestFromMomentum_AllFields(t *testing.T) {
	m := &models.Momentum{
		Height: 100, Hash: "abc", Timestamp: 1700000000, TxCount: 5,
		Producer: "z1qp", ProducerOwner: "z1qo", ProducerName: "alphanet-1",
	}
	d := FromMomentum(m)
	if d.Height != 100 || d.Hash != "abc" || d.Timestamp != 1700000000 ||
		d.TxCount != 5 || d.Producer != "z1qp" ||
		d.ProducerOwner != "z1qo" || d.ProducerName != "alphanet-1" {
		t.Errorf("FromMomentum mapped fields wrong: %+v", d)
	}
}

func TestFromMomentums_EmptySliceNotNull(t *testing.T) {
	got := FromMomentums(nil)
	if got == nil {
		t.Fatal("FromMomentums(nil) returned nil; want empty slice")
	}
	b, _ := json.Marshal(got)
	if string(b) != "[]" {
		t.Errorf("JSON of empty slice = %s, want []", b)
	}
}

// TestMomentum_AllModelFieldsCovered ensures every exported field on
// models.Momentum is consumed by FromMomentum. If someone adds a column
// to the schema and forgets the DTO mapping, this test fails.
func TestMomentum_AllModelFieldsCovered(t *testing.T) {
	mt := reflect.TypeOf(models.Momentum{})
	dt := reflect.TypeOf(Momentum{})

	dtoFields := map[string]bool{}
	for i := 0; i < dt.NumField(); i++ {
		dtoFields[dt.Field(i).Name] = true
	}
	for i := 0; i < mt.NumField(); i++ {
		name := mt.Field(i).Name
		if !dtoFields[name] {
			t.Errorf("models.Momentum.%s is not represented on dto.Momentum (add a json field + FromMomentum mapping, or rename)", name)
		}
	}
}

func TestMomentum_JSONShape(t *testing.T) {
	d := FromMomentum(&models.Momentum{Height: 42, Hash: "abc", Timestamp: 0, TxCount: 0, Producer: "p"})
	b, _ := json.Marshal(d)
	for _, want := range []string{
		`"height":42`, `"hash":"abc"`, `"timestamp":0`, `"tx_count":0`, `"producer":"p"`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("JSON %s missing %q", b, want)
		}
	}
	// omitempty: empty producer_owner / producer_name should not appear.
	if strings.Contains(string(b), "producer_owner") || strings.Contains(string(b), "producer_name") {
		t.Errorf("expected omitempty to drop producer_owner / producer_name in %s", b)
	}
}
