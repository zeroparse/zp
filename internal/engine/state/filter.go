package state

// pushFiltered keeps original node values in input order. The predicate must
// return an actual bool; truthiness and static type narrowing are not implied.
func (c *EvalCtx) pushFiltered(args []any) error {
	f, ok := args[1].(Function)
	if !ok || f.Arity() != 1 {
		return c.P.ActionError("filter: second argument must be a function of one parameter")
	}
	items, err := c.P.ValueListItems("filter", args[0])
	if err != nil {
		return err
	}
	for i := range items {
		c.P.Spend()
		val, err := f.Apply(c.P.termItemAt(items, i, false), nil)
		if err != nil {
			return err
		}
		keep, err := c.P.termFilterKeep(val)
		if err != nil {
			return err
		}
		if keep {
			c.P.KidStack = append(c.P.KidStack, items[i])
		}
	}
	return nil
}
