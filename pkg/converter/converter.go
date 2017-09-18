package converter

// TODO try using jsonparser, assume that root is a map
// build a nested map without values to track the location
// try to write some tests first
// TODO also consider https://godoc.org/github.com/json-iterator/go
// it should have similar methods, it'd better cause it's what
// Kubernetes uses, and Tim has contributed a ton of tests etc :)

import (
	"encoding/json"
	"fmt"

	"github.com/buger/jsonparser"
)

type branchInfo struct {
	kind   jsonparser.ValueType
	self   branch
	value  []byte
	parent *branchInfo
	path   branchPath
}

type branch = map[string]*branchInfo
type branchPath = []string

type Converter struct {
	tree branchInfo
	data []byte
	// TODO add module context, cause we want to be able to lookup variables etc
	keywords map[string]keywordHandler
}

type keywordHandler func(branch *branchInfo) error

func New() *Converter {
	c := &Converter{
		keywords: map[string]keywordHandler{
			"kubegen.MapMerge.Lookup": func(branch *branchInfo) error {
				switch branch.kind {
				case jsonparser.String:
					// evidently using Delete on this segment corrupts the rest of the data
					// we need to track our path and operate on the top-level c.data
					branch.parent.value = jsonparser.Delete(branch.parent.value, "kubegen.MapMerge.Lookup")
				}
				return nil
			},
		},
	}
	return c
}

func (c *Converter) Load(data []byte) {
	c.data = data
}

// LoadStrict ensure we validate the data, as out jsonparser doesn't produce very helpful errors,
// namely it doesn't indicate line numbers and seems to tolerate leading commas etc
func (c *Converter) LoadStrict(data []byte) error {
	obj := new(interface{})
	if err := json.Unmarshal(data, obj); err != nil {
		return fmt.Errorf(
			"error while re-decoding %q (%q): %v",
			"<TODO:instanceName>", "<TODO:sourcePath", err)
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf(
			"error while re-encoding %q (%q): %v",
			"<TODO:instanceName>", "<TODO:sourcePath>", err)
	}
	c.data = data
	return nil
}

func (c *Converter) LoadParsed(obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf(
			"error while re-encoding %q (%q): %v",
			"<TODO:instanceName>", "<TODO:sourcePath>", err)
	}
	c.data = data
	return nil
}

func (c *Converter) doIterate(parentBranch *branchInfo, key string, value []byte, dataType jsonparser.ValueType) {
	pathLen := len(parentBranch.path) + 1
	newBranch := branchInfo{
		parent: parentBranch,
		kind:   dataType,
		value:  value,
		self:   make(branch),
		path:   make(branchPath, pathLen),
	}

	// we have to copy the slice explicitly, as it turns append doesn't make a copy
	copy(newBranch.path, parentBranch.path)
	newBranch.path[pathLen-1] = key

	if _, ok := parentBranch.self[key]; ok {
		panic(fmt.Sprintf("key %q is already set in parent", key))
	}
	parentBranch.self[key] = &newBranch

	//if handler, ok := c.keywords[key]; ok {
	//	handler(&newBranch)
	//	return
	//}

	switch dataType {
	case jsonparser.Object:
		handler := c.makeObjectIterator(&newBranch)

		if err := jsonparser.ObjectEach(value, handler); err != nil {
			panic(err)
		}
	case jsonparser.Array:
		handler := c.makeArrayIterator(&newBranch)

		jsonparser.ArrayEach(value, handler)
	}
}

type objectIterator func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error

func (c *Converter) makeObjectIterator(parentBranch *branchInfo) objectIterator {
	callback := func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		c.doIterate(parentBranch, string(key), value, dataType)
		keys := []string{}
		paths := [][]string{}
		for i := range parentBranch.self {
			keys = append(keys, i)
			paths = append(paths, parentBranch.self[i].path)
		}
		return nil
	}
	return callback
}

type arrayIterator func(value []byte, dataType jsonparser.ValueType, offset int, err error)

func (c *Converter) makeArrayIterator(parentBranch *branchInfo) arrayIterator {
	index := 0
	callback := func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		c.doIterate(parentBranch, fmt.Sprintf("[[%d]]", index), value, dataType)
		index = index + 1
	}
	return callback
}

func (c *Converter) Run() error {
	kind, err := jsonparser.GetString(c.data, "Kind")
	if err != nil {
		return err
	}
	if kind == "" {
		// it is okay to check this here, so we fail early,
		// however we should abstract deep checks more specifically
		return fmt.Errorf("kind is blank")
	}

	c.tree = branchInfo{
		parent: nil,
		kind:   jsonparser.Object,
		value:  c.data,
		self:   make(branch),
		path:   []string{""},
	}

	if err := jsonparser.ObjectEach(c.data, c.makeObjectIterator(&c.tree)); err != nil {
		return err
	}
	return nil
}

func (b *branchInfo) get(key string) *branchInfo {
	if next := b.self[key]; next != nil {
		return next
	}
	return nil
}

func (c *Converter) get(path ...string) *branchInfo {
	branch := branchInfo{self: c.tree.self}
	for x := range path {
		if next := branch.get(path[x]); next != nil {
			branch = *next
		} else {
			return nil
		}
	}
	return &branch
}
