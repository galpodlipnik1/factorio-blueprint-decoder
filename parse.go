package bpdecode

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type storageVersion struct {
	Major uint16
	Minor uint16
	Patch uint16
	Build uint16
}

func (v storageVersion) AtLeast(major uint16, minor uint16, patch uint16, build uint16) bool {
	left := [4]uint16{v.Major, v.Minor, v.Patch, v.Build}
	right := [4]uint16{major, minor, patch, build}
	for i := 0; i < len(left); i++ {
		if left[i] > right[i] {
			return true
		}
		if left[i] < right[i] {
			return false
		}
	}
	return true
}

type signalKind byte

const (
	signalItem signalKind = iota
	signalFluid
	signalVirtual
)

type prototypeIndex struct {
	items    map[uint16]string
	fluids   map[uint16]string
	virtuals map[uint16]string
}

func newPrototypeIndex() *prototypeIndex {
	return &prototypeIndex{
		items:    make(map[uint16]string),
		fluids:   make(map[uint16]string),
		virtuals: make(map[uint16]string),
	}
}

func (p *prototypeIndex) add(prototype string, id uint16, name string) {
	switch prototypeCategory(prototype) {
	case signalItem:
		p.items[id] = name
	case signalFluid:
		p.fluids[id] = name
	case signalVirtual:
		p.virtuals[id] = name
	}
}

func (p *prototypeIndex) resolve(kind signalKind, id uint16) string {
	switch kind {
	case signalItem:
		return p.items[id]
	case signalFluid:
		return p.fluids[id]
	case signalVirtual:
		return p.virtuals[id]
	default:
		return ""
	}
}

func prototypeCategory(prototype string) signalKind {
	switch prototype {
	case "fluid":
		return signalFluid
	case "virtual-signal":
		return signalVirtual
	default:
		return signalItem
	}
}

type libraryNode struct {
	Kind        string
	Slot        int
	PathValid   bool
	Label       string
	Description string
	IconSprite  string
	EntityCount int
	Children    []libraryNode
}

type byteStream struct {
	data []byte
	pos  int
}

func newByteStream(data []byte) *byteStream {
	return &byteStream{data: data}
}

func (s *byteStream) position() int {
	return s.pos
}

func (s *byteStream) seek(position int) error {
	if position < 0 || position > len(s.data) {
		return fmt.Errorf("seek to invalid position %d", position)
	}
	s.pos = position
	return nil
}

func (s *byteStream) skip(count int) error {
	return s.seek(s.pos + count)
}

func (s *byteStream) readBytes(count int) ([]byte, error) {
	if count < 0 || s.pos+count > len(s.data) {
		return nil, fmt.Errorf("unexpected EOF at %d while reading %d bytes", s.pos, count)
	}
	out := s.data[s.pos : s.pos+count]
	s.pos += count
	return out, nil
}

func (s *byteStream) u8() (uint8, error) {
	bytes, err := s.readBytes(1)
	if err != nil {
		return 0, err
	}
	return bytes[0], nil
}

func (s *byteStream) u16() (uint16, error) {
	bytes, err := s.readBytes(2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(bytes), nil
}

func (s *byteStream) u32() (uint32, error) {
	bytes, err := s.readBytes(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(bytes), nil
}

func (s *byteStream) s32() (int32, error) {
	value, err := s.u32()
	return int32(value), err
}

func (s *byteStream) bool() (bool, error) {
	value, err := s.u8()
	if err != nil {
		return false, err
	}
	if value != 0 && value != 1 {
		return false, fmt.Errorf("invalid bool %d at %d", value, s.pos-1)
	}
	return value == 1, nil
}

func (s *byteStream) count() (uint32, error) {
	length, err := s.u8()
	if err != nil {
		return 0, err
	}
	if length == 0xff {
		return s.u32()
	}
	return uint32(length), nil
}

func (s *byteStream) string() (string, error) {
	length, err := s.count()
	if err != nil {
		return "", err
	}
	bytes, err := s.readBytes(int(length))
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (s *byteStream) expect(expected ...byte) error {
	for _, want := range expected {
		got, err := s.u8()
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("expected 0x%02x, got 0x%02x at %d", want, got, s.pos-1)
		}
	}
	return nil
}

func ParseBlueprintLibrary(data []byte) ([]Entry, error) {
	stream := newByteStream(data)

	version, err := parseVersion(stream)
	if err != nil {
		return nil, fmt.Errorf("read library version: %w", err)
	}
	if err := stream.expect(0x00); err != nil {
		return nil, fmt.Errorf("read library separator: %w", err)
	}
	if err := skipMigrations(stream); err != nil {
		return nil, fmt.Errorf("skip migrations: %w", err)
	}

	index, err := skipIndex(stream, version)
	if err != nil {
		return nil, fmt.Errorf("skip prototype index: %w", err)
	}

	if err := stream.skip(1); err != nil {
		return nil, fmt.Errorf("skip library state: %w", err)
	}
	if err := stream.expect(0x00); err != nil {
		return nil, fmt.Errorf("read library separator 2: %w", err)
	}
	if _, err := stream.u32(); err != nil {
		return nil, fmt.Errorf("read generation counter: %w", err)
	}
	if _, err := stream.u32(); err != nil {
		return nil, fmt.Errorf("read timestamp: %w", err)
	}
	if version.Major >= 2 {
		if _, err := stream.u32(); err != nil {
			return nil, fmt.Errorf("read library reserved field: %w", err)
		}
	}
	if err := stream.expect(0x01); err != nil {
		return nil, fmt.Errorf("read library object marker: %w", err)
	}

	nodes, err := parseLibraryObjects(stream, index, version, 1, true)
	if err != nil {
		return nil, err
	}
	nodes = append(nodes, recoverEmbeddedObjects(data, stream.position(), index, version, maxTopLevelSlot(nodes))...)

	entries := flattenNodes(nodes)
	if len(entries) == 0 {
		return nil, errors.New("no blueprint or blueprint-book entries found")
	}

	return entries, nil
}

func maxTopLevelSlot(nodes []libraryNode) int {
	maxSlot := 0
	for _, node := range nodes {
		if node.Slot > maxSlot {
			maxSlot = node.Slot
		}
	}
	return maxSlot
}

func parseVersion(stream *byteStream) (storageVersion, error) {
	major, err := stream.u16()
	if err != nil {
		return storageVersion{}, err
	}
	minor, err := stream.u16()
	if err != nil {
		return storageVersion{}, err
	}
	patch, err := stream.u16()
	if err != nil {
		return storageVersion{}, err
	}
	build, err := stream.u16()
	if err != nil {
		return storageVersion{}, err
	}
	return storageVersion{Major: major, Minor: minor, Patch: patch, Build: build}, nil
}

func skipMigrations(stream *byteStream) error {
	count, err := stream.u8()
	if err != nil {
		return err
	}
	for i := 0; i < int(count); i++ {
		if _, err := stream.string(); err != nil {
			return err
		}
		if _, err := stream.string(); err != nil {
			return err
		}
	}
	return nil
}

func skipIndex(stream *byteStream, version storageVersion) (*prototypeIndex, error) {
	index := newPrototypeIndex()

	prototypeCount, err := stream.u16()
	if err != nil {
		return nil, err
	}

	for i := 0; i < int(prototypeCount); i++ {
		prototypeName, err := stream.string()
		if err != nil {
			return nil, err
		}

		useWideIDs := usesWidePrototypeIDs(version, prototypeName)
		var nameCount uint16
		if useWideIDs {
			nameCount, err = stream.u16()
		} else {
			count8, readErr := stream.u8()
			err = readErr
			nameCount = uint16(count8)
		}
		if err != nil {
			return nil, err
		}

		for j := 0; j < int(nameCount); j++ {
			var id uint16
			if useWideIDs {
				id, err = stream.u16()
			} else {
				id8, readErr := stream.u8()
				err = readErr
				id = uint16(id8)
			}
			if err != nil {
				return nil, err
			}

			name, err := stream.string()
			if err != nil {
				return nil, err
			}
			index.add(prototypeName, id, name)
		}
	}

	return index, nil
}

func usesWidePrototypeIDs(version storageVersion, prototypeName string) bool {
	if prototypeName == "quality" {
		return false
	}
	if version.Major >= 2 {
		return true
	}
	return prototypeName != "tile"
}

func parseLibraryObjects(stream *byteStream, index *prototypeIndex, version storageVersion, slotBase int, pathValid bool) ([]libraryNode, error) {
	objectCount, err := stream.u32()
	if err != nil {
		return nil, err
	}

	nodes := make([]libraryNode, 0)
	for slot := 0; slot < int(objectCount); slot++ {
		used, err := stream.bool()
		if err != nil {
			return nil, fmt.Errorf("read library slot %d used flag: %w", slot+1, err)
		}
		if !used {
			continue
		}

		prefix, err := stream.u8()
		if err != nil {
			return nil, fmt.Errorf("read library slot %d prefix: %w", slot+1, err)
		}
		if _, err := stream.u32(); err != nil {
			return nil, fmt.Errorf("read library slot %d generation: %w", slot+1, err)
		}
		if _, err := stream.u16(); err != nil {
			return nil, fmt.Errorf("read library slot %d item id: %w", slot+1, err)
		}

		switch prefix {
		case 0:
			node, err := parseBlueprint(stream, index, version)
			if err != nil {
				return nil, fmt.Errorf("parse blueprint at slot %d: %w", slot+1, err)
			}
			node.Slot = slot + slotBase
			node.PathValid = pathValid
			nodes = append(nodes, node)
		case 1:
			node, err := parseBlueprintBook(stream, index, version, pathValid)
			if err != nil {
				return nil, fmt.Errorf("parse blueprint book at slot %d: %w", slot+1, err)
			}
			node.Slot = slot + slotBase
			node.PathValid = pathValid
			nodes = append(nodes, node)
		case 2:
			if err := skipDeconstructionPlanner(stream, version); err != nil {
				return nil, fmt.Errorf("skip deconstruction planner at slot %d: %w", slot+1, err)
			}
		case 3:
			if err := skipUpgradePlanner(stream, version); err != nil {
				return nil, fmt.Errorf("skip upgrade planner at slot %d: %w", slot+1, err)
			}
		default:
			return nil, fmt.Errorf("unknown library object prefix %d at %d", prefix, stream.position()-1)
		}
	}

	return nodes, nil
}

func parseBlueprint(stream *byteStream, index *prototypeIndex, libraryVersion storageVersion) (libraryNode, error) {
	label, err := stream.string()
	if err != nil {
		return libraryNode{}, err
	}
	if err := stream.expect(0x00); err != nil {
		return libraryNode{}, err
	}
	hasRemovedMods, err := stream.bool()
	if err != nil {
		return libraryNode{}, err
	}
	contentSize, err := stream.count()
	if err != nil {
		return libraryNode{}, err
	}
	contentStart := stream.position()
	contentEnd := contentStart + int(contentSize)
	if contentEnd < contentStart || contentEnd > len(stream.data) {
		return libraryNode{}, fmt.Errorf("invalid blueprint content range %d:%d", contentStart, contentEnd)
	}

	description, entityCount, iconSprite := parseBlueprintContent(stream.data[contentStart:contentEnd], index, libraryVersion)
	if iconSprite == "" {
		iconSprite = iconFromLabel(label)
	}

	if err := stream.seek(contentEnd); err != nil {
		return libraryNode{}, err
	}

	if hasRemovedMods {
		localIndexSize, err := stream.count()
		if err != nil {
			return libraryNode{}, err
		}
		if err := stream.skip(int(localIndexSize)); err != nil {
			return libraryNode{}, err
		}
	}

	return libraryNode{
		Kind:        "blueprint",
		Label:       label,
		Description: description,
		IconSprite:  iconSprite,
		EntityCount: entityCount,
	}, nil
}

type recoveredObject struct {
	node      libraryNode
	endOffset int
	fullRange bool
}

func recoverEmbeddedObjects(data []byte, start int, index *prototypeIndex, version storageVersion, slotBase int) []libraryNode {
	recovered := make([]libraryNode, 0)
	nextSlot := slotBase + 1
	skipUntil := start

	for offset := start; offset+8 < len(data); offset++ {
		if offset < skipUntil {
			continue
		}
		if data[offset] != 0x01 {
			continue
		}

		prefix := data[offset+1]
		if prefix > 1 {
			continue
		}

		recoveredObject, ok := tryRecoverEmbeddedObject(data, offset, index, version)
		if !ok || !looksLikeUserText(recoveredObject.node.Label) {
			continue
		}

		recoveredObject.node.Slot = nextSlot
		recoveredObject.node.PathValid = false
		nextSlot++
		recovered = append(recovered, recoveredObject.node)

		if recoveredObject.fullRange && recoveredObject.endOffset > offset {
			skipUntil = recoveredObject.endOffset
		}
	}

	return recovered
}

func tryRecoverEmbeddedObject(data []byte, offset int, index *prototypeIndex, version storageVersion) (recoveredObject, bool) {
	stream := newByteStream(data)
	if err := stream.seek(offset); err != nil {
		return recoveredObject{}, false
	}

	used, err := stream.bool()
	if err != nil || !used {
		return recoveredObject{}, false
	}

	prefix, err := stream.u8()
	if err != nil {
		return recoveredObject{}, false
	}
	if prefix > 1 {
		return recoveredObject{}, false
	}

	if _, err := stream.u32(); err != nil {
		return recoveredObject{}, false
	}

	if _, err := stream.u16(); err != nil {
		return recoveredObject{}, false
	}

	objectStart := stream.position()
	if prefix == 0 {
		node, err := parseBlueprint(stream, index, version)
		if err != nil || !looksLikeUserText(node.Label) || !blueprintNodeLooksValid(data, objectStart, stream.position()) {
			return recoveredObject{}, false
		}
		return recoveredObject{
			node:      node,
			endOffset: stream.position(),
			fullRange: true,
		}, true
	}

	node, err := parseBlueprintBook(stream, index, version, false)
	if err == nil && looksLikeUserText(node.Label) {
		return recoveredObject{
			node:      node,
			endOffset: stream.position(),
			fullRange: true,
		}, true
	}

	if err := stream.seek(objectStart); err != nil {
		return recoveredObject{}, false
	}

	node, err = parseBlueprintBookShallow(stream, index, version)
	if err != nil || !looksLikeUserText(node.Label) {
		return recoveredObject{}, false
	}
	node.PathValid = false

	return recoveredObject{node: node}, true
}

func parseBlueprintBookShallow(stream *byteStream, index *prototypeIndex, version storageVersion) (libraryNode, error) {
	label, err := stream.string()
	if err != nil {
		return libraryNode{}, err
	}
	description, err := stream.string()
	if err != nil {
		return libraryNode{}, err
	}
	iconSprite, err := parseIcons(stream, index, version)
	if err != nil {
		return libraryNode{}, err
	}
	childCount, err := stream.u32()
	if err != nil {
		return libraryNode{}, err
	}
	if childCount == 0 || childCount > 4096 {
		return libraryNode{}, fmt.Errorf("implausible embedded book child count %d", childCount)
	}

	return libraryNode{
		Kind:        "blueprint-book",
		Label:       label,
		Description: description,
		IconSprite:  iconSprite,
	}, nil
}

func blueprintNodeLooksValid(data []byte, objectStart int, objectEnd int) bool {
	stream := newByteStream(data)
	if err := stream.seek(objectStart); err != nil {
		return false
	}

	if _, err := stream.string(); err != nil {
		return false
	}
	if err := stream.expect(0x00); err != nil {
		return false
	}
	if _, err := stream.bool(); err != nil {
		return false
	}
	contentSize, err := stream.count()
	if err != nil {
		return false
	}

	contentStart := stream.position()
	contentEnd := contentStart + int(contentSize)
	if contentStart >= len(data) || contentEnd > len(data) || contentEnd > objectEnd {
		return false
	}

	content := newByteStream(data[contentStart:contentEnd])
	version, err := parseVersion(content)
	if err != nil {
		return false
	}
	if version.Major == 0 {
		return false
	}
	if err := content.expect(0x00); err != nil {
		return false
	}
	if err := skipMigrations(content); err != nil {
		return false
	}

	return true
}

func looksLikeUserText(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}

	hasGraphic := false
	for _, r := range value {
		if r == utf8.RuneError {
			return false
		}
		if unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
		if unicode.IsGraphic(r) {
			hasGraphic = true
		}
	}

	return hasGraphic
}

func parseBlueprintContent(data []byte, index *prototypeIndex, version storageVersion) (description string, entityCount int, iconSprite string) {
	iconSprite = scanIconsFromEnd(data, index, version)
	return
}

func skipSnapToGrid(s *byteStream) error {
	hasGrid, err := s.bool()
	if err != nil {
		return err
	}
	if !hasGrid {
		return nil
	}
	if _, err := s.u32(); err != nil {
		return err
	}
	if _, err := s.u32(); err != nil {
		return err
	}
	hasPosition, err := s.bool()
	if err != nil {
		return err
	}
	if hasPosition {
		if _, err := s.s32(); err != nil {
			return err
		}
		if _, err := s.s32(); err != nil {
			return err
		}
	}
	return nil
}

func scanIconsFromEnd(data []byte, index *prototypeIndex, version storageVersion) string {
	iconEntrySize := 3
	if version.Major >= 2 {
		iconEntrySize = 4
	}

	for trail := 0; trail <= 8; trail++ {
		for iconCount := 4; iconCount >= 1; iconCount-- {
			sectionSize := 2 + iconCount*iconEntrySize
			offset := len(data) - sectionSize - trail
			if offset < 0 {
				continue
			}

			if data[offset] != 0x00 {
				continue
			}
			if int(data[offset+1]) != iconCount {
				continue
			}

			valid := true
			firstSprite := ""
			pos := offset + 2
			for i := 0; i < iconCount; i++ {
				typeID := data[pos]
				if typeID > 2 {
					valid = false
					break
				}
				nameID := binary.LittleEndian.Uint16(data[pos+1:])
				pos += iconEntrySize

				if firstSprite != "" {
					continue
				}
				name := index.resolve(signalKind(typeID), nameID)
				if name == "" {
					continue
				}
				switch signalKind(typeID) {
				case signalItem:
					firstSprite = "item/" + name
				case signalFluid:
					firstSprite = "fluid/" + name
				case signalVirtual:
					firstSprite = "virtual-signal/" + name
				}
			}

			if valid && firstSprite != "" {
				return firstSprite
			}
		}
	}

	return ""
}

func iconFromLabel(label string) string {
	for i := 0; i < len(label); {
		start := strings.IndexByte(label[i:], '[')
		if start == -1 {
			break
		}
		start += i
		end := strings.IndexByte(label[start:], ']')
		if end == -1 {
			break
		}
		end += start
		tag := label[start+1 : end]
		eq := strings.IndexByte(tag, '=')
		if eq != -1 {
			tagType := tag[:eq]
			tagName := tag[eq+1:]
			switch tagType {
			case "item":
				return "item/" + tagName
			case "fluid":
				return "fluid/" + tagName
			case "virtual-signal":
				return "virtual-signal/" + tagName
			}
		}
		i = end + 1
	}
	return ""
}

func parseBlueprintBook(stream *byteStream, index *prototypeIndex, version storageVersion, pathValid bool) (libraryNode, error) {
	label, err := stream.string()
	if err != nil {
		return libraryNode{}, err
	}
	description, err := stream.string()
	if err != nil {
		return libraryNode{}, err
	}
	iconSprite, err := parseIcons(stream, index, version)
	if err != nil {
		return libraryNode{}, err
	}
	children, err := parseLibraryObjects(stream, index, version, 0, pathValid)
	if err != nil {
		return libraryNode{}, err
	}
	if _, err := stream.u8(); err != nil {
		return libraryNode{}, err
	}
	if err := stream.expect(0x00); err != nil {
		return libraryNode{}, err
	}

	return libraryNode{
		Kind:        "blueprint-book",
		Label:       label,
		Description: description,
		IconSprite:  iconSprite,
		Children:    children,
	}, nil
}

func parseIcons(stream *byteStream, index *prototypeIndex, version storageVersion) (string, error) {
	unknownCount, err := stream.u8()
	if err != nil {
		return "", err
	}
	unknownNames := make([]string, 0, int(unknownCount))
	for i := 0; i < int(unknownCount); i++ {
		name, err := stream.string()
		if err != nil {
			return "", err
		}
		unknownNames = append(unknownNames, name)
	}

	iconCount, err := stream.u8()
	if err != nil {
		return "", err
	}

	var firstSprite string
	for i := 0; i < int(iconCount); i++ {
		typeID, err := stream.u8()
		if err != nil {
			return "", err
		}
		nameID, err := stream.u16()
		if err != nil {
			return "", err
		}
		if version.Major >= 2 {
			if _, err := stream.u8(); err != nil {
				return "", err
			}
		}

		if firstSprite != "" {
			continue
		}

		kind := signalKind(typeID)
		name := ""
		if i < len(unknownNames) && unknownNames[i] != "" {
			name = unknownNames[i]
		} else {
			name = index.resolve(kind, nameID)
		}
		if name == "" {
			continue
		}

		switch kind {
		case signalItem:
			firstSprite = "item/" + name
		case signalFluid:
			firstSprite = "fluid/" + name
		case signalVirtual:
			firstSprite = "virtual-signal/" + name
		}
	}

	return firstSprite, nil
}

func skipDeconstructionPlanner(stream *byteStream, version storageVersion) error {
	if _, err := stream.string(); err != nil {
		return err
	}
	if _, err := stream.string(); err != nil {
		return err
	}
	if _, err := parseIcons(stream, newPrototypeIndex(), version); err != nil {
		return err
	}
	if _, err := stream.u8(); err != nil {
		return err
	}
	if err := skipUnknownFilters(stream); err != nil {
		return err
	}
	if err := skipFilterEntries(stream, version, false); err != nil {
		return err
	}
	if _, err := stream.bool(); err != nil {
		return err
	}
	if _, err := stream.u8(); err != nil {
		return err
	}
	if _, err := stream.u8(); err != nil {
		return err
	}
	if err := skipUnknownFilters(stream); err != nil {
		return err
	}
	return skipFilterEntries(stream, version, true)
}

func skipUpgradePlanner(stream *byteStream, version storageVersion) error {
	if _, err := stream.string(); err != nil {
		return err
	}
	if _, err := stream.string(); err != nil {
		return err
	}
	if _, err := parseIcons(stream, newPrototypeIndex(), version); err != nil {
		return err
	}

	unknownCount, err := stream.u8()
	if err != nil {
		return err
	}
	for i := 0; i < int(unknownCount); i++ {
		if _, err := stream.string(); err != nil {
			return err
		}
		if _, err := stream.bool(); err != nil {
			return err
		}
		if _, err := stream.u16(); err != nil {
			return err
		}
	}

	mapperCount, err := stream.u8()
	if err != nil {
		return err
	}
	for i := 0; i < int(mapperCount); i++ {
		if err := skipUpgradeMapper(stream); err != nil {
			return err
		}
		if err := skipUpgradeMapper(stream); err != nil {
			return err
		}
	}

	return nil
}

func skipUnknownFilters(stream *byteStream) error {
	unknownCount, err := stream.u8()
	if err != nil {
		return err
	}
	for i := 0; i < int(unknownCount); i++ {
		if _, err := stream.u16(); err != nil {
			return err
		}
		if _, err := stream.string(); err != nil {
			return err
		}
	}
	return nil
}

func skipFilterEntries(stream *byteStream, version storageVersion, tile bool) error {
	filterCount, err := stream.u8()
	if err != nil {
		return err
	}
	for i := 0; i < int(filterCount); i++ {
		if tile && version.Major < 2 {
			if _, err := stream.u8(); err != nil {
				return err
			}
			continue
		}
		if _, err := stream.u16(); err != nil {
			return err
		}
		if version.Major >= 2 && !tile {
			if _, err := stream.u8(); err != nil {
				return err
			}
		}
	}
	return nil
}

func skipUpgradeMapper(stream *byteStream) error {
	if _, err := stream.u8(); err != nil {
		return err
	}
	_, err := stream.u16()
	return err
}

func flattenNodes(nodes []libraryNode) []Entry {
	entries := make([]Entry, 0)
	for _, node := range nodes {
		flattenNode(&entries, node, nil, nil, "")
	}
	return entries
}

func flattenNode(entries *[]Entry, node libraryNode, path []int, breadcrumbs []string, parentPathKey string) string {
	currentPath := []int{}
	pathKey := ""
	if node.PathValid {
		currentPath = append(copyInts(path), node.Slot)
		pathKey = joinPath(currentPath)
	} else if parentPathKey == "" {
		pathKey = fmt.Sprintf("recovered:%d", node.Slot)
	} else {
		pathKey = parentPathKey + "/" + strconv.Itoa(node.Slot)
	}

	name := fallbackName(node.Label)
	currentBreadcrumbs := append(copyStrings(breadcrumbs), name)
	breadcrumb := stringsJoin(currentBreadcrumbs, " / ")

	entry := Entry{
		Path:              currentPath,
		PathKey:           pathKey,
		ParentPathKey:     parentPathKey,
		RecordType:        node.Kind,
		Name:              name,
		Description:       node.Description,
		Breadcrumb:        breadcrumb,
		SearchName:        normalize(name),
		SearchDescription: normalize(node.Description),
		SearchBreadcrumb:  normalize(breadcrumb),
		LabelResolved:     true,
		ChildPathKeys:     []string{},
		IconSprite:        node.IconSprite,
		EntityCount:       node.EntityCount,
		Tags:              map[string]any{},
	}
	entry.SearchText = buildSearchText(entry.Name, entry.Description, entry.Breadcrumb, entry.Tags)

	entryIndex := len(*entries)
	*entries = append(*entries, entry)

	for _, child := range node.Children {
		childPathKey := flattenNode(entries, child, currentPath, currentBreadcrumbs, pathKey)
		(*entries)[entryIndex].ChildPathKeys = append((*entries)[entryIndex].ChildPathKeys, childPathKey)
	}

	return pathKey
}
