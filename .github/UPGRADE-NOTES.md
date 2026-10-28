<!--
What the next release asks of a repo that upgrades seams to it, in plain
words. The release workflow puts whatever follows this comment above the
release's changelog, and adds nothing when there is nothing here. Empty it,
down to this comment, once the release is out. Head the notes "## Upgrading",
level with the changelog's own "## Changelog".
-->
<!-- markdownlint-disable-file MD041 -->

## Upgrading

After upgrading, `seams render --check` fails once. Run `seams render` to fix
it: the register gains a line that links to the guide to reading it. Commit
the rendered register, and the check passes again.
