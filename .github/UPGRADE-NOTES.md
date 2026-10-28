<!--
What the next release asks of a repo that upgrades seams to it, in plain
words. The release workflow puts whatever follows this comment above the
release's changelog, and adds nothing when there is nothing here. Empty it,
down to this comment, once a final release is out, not after a pre-release,
so that every pre-release and the final release after them carry the notes.
Head the notes "## Upgrading",
level with the changelog's own "## Changelog".
-->
<!-- markdownlint-disable-file MD041 -->

## Upgrading

sheepdog-seams is now seams, at `bitshepherds/seams`, and it keeps each
repo's files in `.seams/`. This version doesn't read the files from before
the rename: until they're moved, every command that reads or writes them
refuses, and says how to move them. So, once, in each repo that uses seams:

1. Install from the new name, with
   `mise use github:bitshepherds/seams@1.0.0`.
2. Move the files: `git mv .sheepdog/seams .seams`.
3. In each file in `.seams/`, change `schema` from
   `https://github.com/bitshepherds/sheepdog-seams/schemas/<name>/v1` to
   `https://github.com/bitshepherds/seams/schemas/<name>/v1`. Nothing else in
   the files changes.
4. Run `seams render`, and commit the register with the moved files. Until
   then, `seams render --check` fails: the register gains a line that links
   to the guide to reading it.

A tool that reads seams' JSON output gets the new schema ids. The schemas
are otherwise unchanged, and still v1.

A tool that writes the status file seams reads with `--status` must now
write the new schema id,
`https://github.com/bitshepherds/seams/schemas/status/v1`. seams refuses a
status file with the old one. Sheepdog, for example, writes that file, and
still writes the old id until bitshepherds/sheepdog#418 moves it to the new
one.
