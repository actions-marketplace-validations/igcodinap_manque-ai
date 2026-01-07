package ast

import (
	"strings"
	"testing"
)

func TestParseGoFile(t *testing.T) {
	parser := NewParser()

	goCode := `package main

import "fmt"

// User represents a user in the system
type User struct {
	ID   int
	Name string
}

// NewUser creates a new user
func NewUser(id int, name string) *User {
	return &User{ID: id, Name: name}
}

func (u *User) GetName() string {
	return u.Name
}

func (u *User) SetName(name string) {
	u.Name = name
}

var GlobalConfig = map[string]string{}

const MaxUsers = 100
`

	symbols, err := parser.ParseFile("main.go", goCode)
	if err != nil {
		t.Fatalf("Failed to parse Go file: %v", err)
	}

	// Check we found the expected symbols
	symbolMap := make(map[string]Symbol)
	for _, s := range symbols {
		symbolMap[s.Name] = s
	}

	// Check struct
	if user, ok := symbolMap["User"]; !ok {
		t.Error("Expected to find User struct")
	} else {
		if user.Kind != SymbolStruct {
			t.Errorf("Expected User to be a struct, got %s", user.Kind)
		}
		if !user.Exported {
			t.Error("Expected User to be exported")
		}
	}

	// Check function
	if newUser, ok := symbolMap["NewUser"]; !ok {
		t.Error("Expected to find NewUser function")
	} else {
		if newUser.Kind != SymbolFunction {
			t.Errorf("Expected NewUser to be a function, got %s", newUser.Kind)
		}
		if len(newUser.Parameters) != 2 {
			t.Errorf("Expected NewUser to have 2 parameters, got %d", len(newUser.Parameters))
		}
	}

	// Check methods
	if getName, ok := symbolMap["GetName"]; !ok {
		t.Error("Expected to find GetName method")
	} else {
		if getName.Kind != SymbolMethod {
			t.Errorf("Expected GetName to be a method, got %s", getName.Kind)
		}
		if getName.Parent != "User" {
			t.Errorf("Expected GetName parent to be User, got %s", getName.Parent)
		}
	}

	// Check variable
	if globalConfig, ok := symbolMap["GlobalConfig"]; !ok {
		t.Error("Expected to find GlobalConfig variable")
	} else {
		if globalConfig.Kind != SymbolVariable {
			t.Errorf("Expected GlobalConfig to be a variable, got %s", globalConfig.Kind)
		}
	}

	// Check constant
	if maxUsers, ok := symbolMap["MaxUsers"]; !ok {
		t.Error("Expected to find MaxUsers constant")
	} else {
		if maxUsers.Kind != SymbolConstant {
			t.Errorf("Expected MaxUsers to be a constant, got %s", maxUsers.Kind)
		}
	}
}

func TestParseTypeScriptFile(t *testing.T) {
	parser := NewParser()

	tsCode := `interface User {
  id: number;
  name: string;
}

export class UserService {
  private users: User[] = [];

  constructor() {}

  public async getUser(id: number): Promise<User | null> {
    return this.users.find(u => u.id === id) || null;
  }

  public addUser(user: User): void {
    this.users.push(user);
  }
}

export function createUser(id: number, name: string): User {
  return { id, name };
}

const DEFAULT_USER: User = { id: 0, name: 'guest' };
`

	symbols, err := parser.ParseFile("user.ts", tsCode)
	if err != nil {
		t.Fatalf("Failed to parse TypeScript file: %v", err)
	}

	symbolMap := make(map[string]Symbol)
	for _, s := range symbols {
		symbolMap[s.Name] = s
	}

	// Check interface
	if user, ok := symbolMap["User"]; !ok {
		t.Error("Expected to find User interface")
	} else {
		if user.Kind != SymbolInterface {
			t.Errorf("Expected User to be an interface, got %s", user.Kind)
		}
	}

	// Check class
	if userService, ok := symbolMap["UserService"]; !ok {
		t.Error("Expected to find UserService class")
	} else {
		if userService.Kind != SymbolClass {
			t.Errorf("Expected UserService to be a class, got %s", userService.Kind)
		}
		if !userService.Exported {
			t.Error("Expected UserService to be exported")
		}
	}

	// Check function
	if createUser, ok := symbolMap["createUser"]; !ok {
		t.Error("Expected to find createUser function")
	} else {
		if createUser.Kind != SymbolFunction {
			t.Errorf("Expected createUser to be a function, got %s", createUser.Kind)
		}
		if !createUser.Exported {
			t.Error("Expected createUser to be exported")
		}
	}

	// Check constant
	if defaultUser, ok := symbolMap["DEFAULT_USER"]; !ok {
		t.Error("Expected to find DEFAULT_USER constant")
	} else {
		if defaultUser.Kind != SymbolConstant {
			t.Errorf("Expected DEFAULT_USER to be a constant, got %s", defaultUser.Kind)
		}
	}
}

func TestParsePythonFile(t *testing.T) {
	parser := NewParser()

	pyCode := `class User:
    """A user in the system."""

    def __init__(self, id: int, name: str):
        self.id = id
        self.name = name

    def get_name(self) -> str:
        return self.name

    def set_name(self, name: str) -> None:
        self.name = name


def create_user(id: int, name: str) -> User:
    """Create a new user."""
    return User(id, name)


async def fetch_user(id: int) -> Optional[User]:
    """Fetch a user from the database."""
    pass


MAX_USERS = 100
DEFAULT_NAME = "guest"
`

	symbols, err := parser.ParseFile("user.py", pyCode)
	if err != nil {
		t.Fatalf("Failed to parse Python file: %v", err)
	}

	symbolMap := make(map[string]Symbol)
	for _, s := range symbols {
		symbolMap[s.Name] = s
	}

	// Check class
	if user, ok := symbolMap["User"]; !ok {
		t.Error("Expected to find User class")
	} else {
		if user.Kind != SymbolClass {
			t.Errorf("Expected User to be a class, got %s", user.Kind)
		}
	}

	// Check function
	if createUser, ok := symbolMap["create_user"]; !ok {
		t.Error("Expected to find create_user function")
	} else {
		if createUser.Kind != SymbolFunction {
			t.Errorf("Expected create_user to be a function, got %s", createUser.Kind)
		}
	}

	// Check async function
	if fetchUser, ok := symbolMap["fetch_user"]; !ok {
		t.Error("Expected to find fetch_user function")
	} else {
		if fetchUser.Kind != SymbolFunction {
			t.Errorf("Expected fetch_user to be a function, got %s", fetchUser.Kind)
		}
	}

	// Check constants
	if maxUsers, ok := symbolMap["MAX_USERS"]; !ok {
		t.Error("Expected to find MAX_USERS constant")
	} else {
		if maxUsers.Kind != SymbolConstant {
			t.Errorf("Expected MAX_USERS to be a constant, got %s", maxUsers.Kind)
		}
	}
}

func TestParseRustFile(t *testing.T) {
	parser := NewParser()

	rustCode := `pub struct User {
    id: u32,
    name: String,
}

impl User {
    pub fn new(id: u32, name: String) -> Self {
        User { id, name }
    }

    pub fn get_name(&self) -> &str {
        &self.name
    }

    fn private_method(&self) {
        // internal use only
    }
}

pub trait UserService {
    fn get_user(&self, id: u32) -> Option<&User>;
    fn add_user(&mut self, user: User);
}

pub fn create_user(id: u32, name: &str) -> User {
    User::new(id, name.to_string())
}

pub const MAX_USERS: u32 = 100;
`

	symbols, err := parser.ParseFile("user.rs", rustCode)
	if err != nil {
		t.Fatalf("Failed to parse Rust file: %v", err)
	}

	symbolMap := make(map[string]Symbol)
	for _, s := range symbols {
		symbolMap[s.Name] = s
	}

	// Check struct
	if user, ok := symbolMap["User"]; !ok {
		t.Error("Expected to find User struct")
	} else {
		if user.Kind != SymbolStruct {
			t.Errorf("Expected User to be a struct, got %s", user.Kind)
		}
		if !user.Exported {
			t.Error("Expected User to be exported (pub)")
		}
	}

	// Check trait
	if userService, ok := symbolMap["UserService"]; !ok {
		t.Error("Expected to find UserService trait")
	} else {
		if userService.Kind != SymbolInterface {
			t.Errorf("Expected UserService to be an interface (trait), got %s", userService.Kind)
		}
	}

	// Check function
	if createUser, ok := symbolMap["create_user"]; !ok {
		t.Error("Expected to find create_user function")
	} else {
		if createUser.Kind != SymbolFunction {
			t.Errorf("Expected create_user to be a function, got %s", createUser.Kind)
		}
	}

	// Check constant
	if maxUsers, ok := symbolMap["MAX_USERS"]; !ok {
		t.Error("Expected to find MAX_USERS constant")
	} else {
		if maxUsers.Kind != SymbolConstant {
			t.Errorf("Expected MAX_USERS to be a constant, got %s", maxUsers.Kind)
		}
	}
}

func TestParseJavaFile(t *testing.T) {
	parser := NewParser()

	javaCode := `package com.example;

public class User {
    private int id;
    private String name;

    public User(int id, String name) {
        this.id = id;
        this.name = name;
    }

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }
}

public interface UserService {
    User getUser(int id);
    void addUser(User user);
}

class InternalHelper {
    static void helper() {}
}
`

	symbols, err := parser.ParseFile("User.java", javaCode)
	if err != nil {
		t.Fatalf("Failed to parse Java file: %v", err)
	}

	symbolMap := make(map[string]Symbol)
	for _, s := range symbols {
		symbolMap[s.Name] = s
	}

	// Check class
	if user, ok := symbolMap["User"]; !ok {
		t.Error("Expected to find User class")
	} else {
		if user.Kind != SymbolClass {
			t.Errorf("Expected User to be a class, got %s", user.Kind)
		}
		if !user.Exported {
			t.Error("Expected User to be exported (public)")
		}
	}

	// Check interface
	if userService, ok := symbolMap["UserService"]; !ok {
		t.Error("Expected to find UserService interface")
	} else {
		if userService.Kind != SymbolInterface {
			t.Errorf("Expected UserService to be an interface, got %s", userService.Kind)
		}
	}

	// Check non-public class
	if helper, ok := symbolMap["InternalHelper"]; !ok {
		t.Error("Expected to find InternalHelper class")
	} else {
		if helper.Exported {
			t.Error("Expected InternalHelper to NOT be exported")
		}
	}
}

func TestGetLanguageFromFilename(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		filename string
		expected string
	}{
		{"main.go", "go"},
		{"app.ts", "typescript"},
		{"app.tsx", "typescript"},
		{"util.js", "javascript"},
		{"component.jsx", "javascript"},
		{"script.py", "python"},
		{"lib.rs", "rust"},
		{"Main.java", "java"},
		{"README.md", ""},
		{"config.yaml", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			lang := parser.GetLanguageFromFilename(tt.filename)
			if lang != tt.expected {
				t.Errorf("GetLanguageFromFilename(%s) = %s, want %s", tt.filename, lang, tt.expected)
			}
		})
	}
}

func TestParseUnsupportedLanguage(t *testing.T) {
	parser := NewParser()

	symbols, err := parser.ParseFile("config.yaml", "key: value")
	if err != nil {
		t.Errorf("Should not error on unsupported language, got: %v", err)
	}
	if len(symbols) != 0 {
		t.Errorf("Expected no symbols for unsupported language, got %d", len(symbols))
	}
}

func TestSymbolLineNumbers(t *testing.T) {
	parser := NewParser()

	goCode := `package main

func First() {}

func Second() {
	// multi-line
	// function
}

func Third() {}
`

	symbols, err := parser.ParseFile("test.go", goCode)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Find Second function
	var second *Symbol
	for _, s := range symbols {
		if s.Name == "Second" {
			second = &s
			break
		}
	}

	if second == nil {
		t.Fatal("Expected to find Second function")
	}

	if second.StartLine < 1 {
		t.Error("StartLine should be positive")
	}

	if second.EndLine < second.StartLine {
		t.Error("EndLine should be >= StartLine")
	}
}

func TestExportDetection(t *testing.T) {
	parser := NewParser()

	goCode := `package main

func PublicFunc() {}

func privateFunc() {}

type PublicType struct {}

type privateType struct {}
`

	symbols, err := parser.ParseFile("test.go", goCode)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	for _, s := range symbols {
		isUpperCase := len(s.Name) > 0 && strings.ToUpper(s.Name[:1]) == s.Name[:1]
		if s.Exported != isUpperCase {
			t.Errorf("Symbol %s: expected Exported=%v, got %v", s.Name, isUpperCase, s.Exported)
		}
	}
}
