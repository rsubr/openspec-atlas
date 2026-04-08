package internal

import "testing"

func TestExtractSQLModelsSkipsConstraintLines(t *testing.T) {
	t.Parallel()

	src := []byte(`
CREATE TABLE users (
  id INTEGER NOT NULL,
  email TEXT,
  PRIMARY KEY (id),
  CONSTRAINT uq_users_email UNIQUE (email)
);
`)

	got := extractSQLModels(src, "schema.sql")
	if len(got) != 1 {
		t.Fatalf("len(extractSQLModels) = %d, want 1", len(got))
	}
	if len(got[0].Fields) != 2 {
		t.Fatalf("len(fields) = %d, want 2", len(got[0].Fields))
	}
	if got[0].Fields[0].Name != "id" || got[0].Fields[0].Nullable {
		t.Fatalf("unexpected id field: %#v", got[0].Fields[0])
	}
}

func TestExtractSQLAlchemyModelsStopsOnDedent(t *testing.T) {
	t.Parallel()

	src := []byte(`class User(Base):
    id = Column(Integer, nullable=False)
    email = Column(String)

def helper():
    return None
`)

	got := extractSQLAlchemyModels(src, "models.py")
	if len(got) != 1 {
		t.Fatalf("len(extractSQLAlchemyModels) = %d, want 1", len(got))
	}
	if got[0].ORM != ORMSQLAlchemy {
		t.Fatalf("ORM = %q, want %q", got[0].ORM, ORMSQLAlchemy)
	}
	if len(got[0].Fields) != 2 {
		t.Fatalf("len(fields) = %d, want 2", len(got[0].Fields))
	}
	if got[0].Fields[0].Name != "id" || got[0].Fields[0].Nullable {
		t.Fatalf("unexpected first field: %#v", got[0].Fields[0])
	}
}
