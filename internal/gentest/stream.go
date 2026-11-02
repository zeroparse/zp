package gentest

const Records = `rule main = ^^ header items:record* #stream $$
rule header = "#records\n"
rule record = ^ k:[a-z]+ #atomic "=" v:[0-9]+ #atomic "\n"`

// RecordsConcat is Records whose elements are struct values with strings concatenated by actions.
const RecordsConcat = `type Rec = { K string; V string }
rule main = ^^ header items:record* #stream $$
rule header = "#records\n"
rule record = ^ k:[a-z]+ #atomic "=" v:[0-9]+ #atomic "\n" => Rec{K: text($k) + "!", V: text($v)}`

// RecordsRecover is Records that recovers from a malformed line by skipping it.
const RecordsRecover = `rule main = ^^ header items:record* #stream $$
rule header = "#records\n"
rule record = ^ (k:[a-z]+ #atomic "=" v:[0-9]+ #atomic "\n") #recover(skip=[^\n]* "\n")`
