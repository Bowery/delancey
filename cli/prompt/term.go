// Copyright 2013-2014 Bowery, Inc.
package prompt

import (
	"Bowery/crosswalk/errors"
	"bufio"
	"io"
)

// prompt reads from in and parses ansi escapes writing to buf.
func (term *Terminal) prompt(buf *Buffer, in io.Reader) (string, error) {
	cols, _, err := TerminalSize(buf.Out)
	if err != nil {
		return "", err
	}
	buf.Cols = cols
	input := bufio.NewReader(in)

	err = buf.Refresh()
	if err != nil {
		return "", errors.NewStackError(err)
	}

	for {
		char, _, err := input.ReadRune()
		if err != nil {
			return buf.String(), errors.NewStackError(err)
		}

		switch char {
		default:
			// Insert characters in the buffer.
			err = buf.Insert(char)
			if err != nil {
				return buf.String(), errors.NewStackError(err)
			}
		case tabKey, ctrlA, ctrlB, ctrlE, ctrlF, ctrlG, ctrlH, ctrlJ, ctrlK, ctrlN,
			ctrlO, ctrlP, ctrlQ, ctrlR, ctrlS, ctrlT, ctrlU, ctrlV, ctrlW, ctrlX,
			ctrlY, ctrlZ:
			// Skip.
			continue
		case returnKey, ctrlD:
			// Finished, return the buffer contents.
			err = buf.EndLine()
			if err != nil {
				err = errors.NewStackError(err)
			}

			return buf.String(), err
		case ctrlC:
			// Finished, return CTRL+C error.
			err = buf.EndLine()
			if err != nil {
				err = errors.NewStackError(err)
			} else {
				err = errors.ErrCTRLC
			}

			return buf.String(), err
		case backKey:
			// Backspace.
			err = buf.DelLeft()
			if err != nil {
				return buf.String(), errors.NewStackError(err)
			}
		case ctrlL:
			// Clear screen.
			err = buf.ClsScreen()
			if err != nil {
				return buf.String(), errors.NewStackError(err)
			}
		case escKey:
			// Functions like arrows, home, etc.
			esc := make([]byte, 2)
			_, err = input.Read(esc)
			if err != nil {
				return buf.String(), errors.NewStackError(err)
			}

			// Home, end.
			if esc[0] == 'O' {
				switch esc[1] {
				case 'H':
					// Home.
					err = buf.Start()
					if err != nil {
						return buf.String(), errors.NewStackError(err)
					}
				case 'F':
					// End.
					err = buf.End()
					if err != nil {
						return buf.String(), errors.NewStackError(err)
					}
				}

				continue
			}

			// Arrows, delete, pgup, pgdown, insert.
			if esc[0] == '[' {
				switch esc[1] {
				case 'A', 'B':
					// Up, down.
					continue
				case 'C':
					// Right.
					err = buf.Right()
					if err != nil {
						return buf.String(), errors.NewStackError(err)
					}
				case 'D':
					// Left.
					err = buf.Left()
					if err != nil {
						return buf.String(), errors.NewStackError(err)
					}
				}

				// Delete, pgup, pgdown, insert.
				if esc[1] > '0' && esc[1] < '7' {
					extEsc := make([]byte, 3)
					_, err = input.Read(extEsc)
					if err != nil {
						return buf.String(), errors.NewStackError(err)
					}

					if extEsc[0] == '~' {
						switch esc[1] {
						case '2', '5', '6':
							// Insert, pgup, pgdown.
							continue
						case '3':
							// Delete.
							err = buf.Del()
							if err != nil {
								return buf.String(), errors.NewStackError(err)
							}
						}
					}
				}
			}
		}
	}

	return buf.String(), nil
}

// password reads from in and parses restricted ansi escapes writing to buf.
func (term *Terminal) password(buf *Buffer, in io.Reader) (string, error) {
	cols, _, err := TerminalSize(buf.Out)
	if err != nil {
		return "", err
	}
	buf.Cols = cols
	input := bufio.NewReader(in)

	err = buf.Refresh()
	if err != nil {
		return "", errors.NewStackError(err)
	}

	for {
		char, _, err := input.ReadRune()
		if err != nil {
			return buf.String(), errors.NewStackError(err)
		}

		switch char {
		default:
			// Insert characters in the buffer.
			err = buf.Insert(char)
			if err != nil {
				return buf.String(), errors.NewStackError(err)
			}
		case tabKey, ctrlA, ctrlB, ctrlE, ctrlF, ctrlG, ctrlJ, ctrlK, ctrlN,
			ctrlO, ctrlP, ctrlQ, ctrlR, ctrlS, ctrlT, ctrlU, ctrlV, ctrlW, ctrlX,
			ctrlY, ctrlZ:
			// Skip.
			continue
		case returnKey, ctrlD:
			// Finished, return the buffer contents.
			err = buf.EndLine()
			if err != nil {
				err = errors.NewStackError(err)
			}

			return buf.String(), err
		case ctrlC:
			// Finished, return CTRL+C error.
			err = buf.EndLine()
			if err != nil {
				err = errors.NewStackError(err)
			} else {
				err = errors.ErrCTRLC
			}

			return buf.String(), err
		case backKey, ctrlH:
			// Backspace.
			err = buf.DelLeft()
			if err != nil {
				return buf.String(), errors.NewStackError(err)
			}
		case ctrlL:
			// Clear screen.
			err = buf.ClsScreen()
			if err != nil {
				return buf.String(), errors.NewStackError(err)
			}
		case escKey:
			// Functions like arrows, home, etc.
			esc := make([]byte, 2)
			_, err = input.Read(esc)
			if err != nil {
				return buf.String(), errors.NewStackError(err)
			}

			// Home, end.
			if esc[0] == 'O' {
				continue
			}

			// Arrows, delete, pgup, pgdown, insert.
			if esc[0] == '[' {
				switch esc[1] {
				case 'A', 'B', 'C', 'D':
					// Up, down, right, left.
					continue
				}

				// Delete, pgup, pgdown, insert.
				if esc[1] > '0' && esc[1] < '7' {
					extEsc := make([]byte, 3)
					_, err = input.Read(extEsc)
					if err != nil {
						return buf.String(), errors.NewStackError(err)
					}

					if extEsc[0] == '~' {
						// Insert, pgup, pgdown, delete.
						continue
					}
				}
			}
		}
	}

	return buf.String(), nil
}
