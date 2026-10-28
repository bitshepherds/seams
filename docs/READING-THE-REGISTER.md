# Reading the register

This guide is for anyone who opens a register, `docs/SEAMS.md`, and wants
to know what it says: a team lead, a product owner, or anyone else who
decides where a team spends its effort. It needs no technical background.

## What a seam is

A **seam** is a place where two pieces of software rely on each other. For
example:

- a shop's checkout calls a payment provider to take a card payment;
- a web app reads prices from the company's own quotes service;
- one team writes a file each night, and another team's system reads it.

One side of a seam provides something, such as the payment service, the
quotes or the file. That side is the **provider**. The other side uses it,
and is a **consumer**. A seam can have more than one consumer.

## Why seams matter

Each side of a seam is usually built and tested by its own team, on its own.
So each team's tests check that its own side works, but not that the two
sides still fit together. If the quotes service renames a field, its tests
still pass, and so do the web app's. Nobody finds out until the two meet in
production, and the web app shows no prices.

Failures like that are expensive. They turn up late, often in front of
customers, and it's not clear whose they are. The register shows every seam
in one place, and how well each is protected, so a team can see where that
could happen before it does.

## The ladder

Each seam is rated on a **ladder**, from rung 0 to rung 6. Each rung adds
one more protection against a change on one side breaking the other, and
builds on the rungs below it.

| Rung | The seam has reached | What it protects against |
| --- | --- | --- |
| 0 | Nothing yet | Nothing. No one has looked at the seam yet. |
| 1 | The seam, its two sides and who uses it are known | A seam nobody knows about. Now someone owns each side, and knows the other side relies on it. |
| 2 | A written, versioned agreement on how the two sides talk, checked for mistakes | Each side working from its own idea of what was agreed. The agreement is called the **contract**. |
| 3 | Each change to the contract checked against the current one and the last release | A change to the contract that would break a side still relying on the old one. |
| 4 | The consumer tested against the contract | The consumer expecting something the contract doesn't promise. |
| 5 | The provider checked against the contract too | The provider no longer doing what the contract promises. With rungs 4 and 5, both sides are tested against the same agreement. |
| 6 | A check before each release that stops one that would break the other side | A release going out that the other side, already live, can't work with. |

## Why some seams top out at 5

Rung 6 is a check that holds back one side's release when it would break the
other side. That only makes sense when the two sides are released at
different times, and a release can be held back. Two kinds of seam aren't
like that, so the highest they can reach, their **top**, is rung 5:

- **A seam whose two sides are released together.** If the web app and the
  quotes service always go out in the same release, they can't get out of
  step, so there's nothing for a check before release to guard.
- **A seam whose provider is an outside vendor.** Take a shop whose checkout
  uses a payment provider. The payment provider publishes the contract, its
  API documentation, and tests its own side against it. The shop tests its
  checkout against that contract. But the payment provider releases on its
  own schedule, and nothing the shop does can hold its release back. The
  payment provider's promise to keep its API stable is the protection there.

A seam at the top of its ladder is as protected as it can be. So a seam that
reads **5 of 5** needs nothing more, just as one that reads **6 of 6**
doesn't.

## How to read the register

### The summary line

The register opens with how many seams there are: how many are **internal**,
and how many **cross out of the system**. An internal seam is between two
parts of the team's own system. A seam crosses out of the system when one of
its sides belongs to someone else, such as a payment provider or a customer,
or when the public can use it.

### The diagram

Under the summary, a diagram shows each part of the system as a box, and each
seam as an arrow from its provider to a consumer. The parts inside the system
are drawn together, and those outside it apart.

### The tables

The internal seams have a table, and so do the seams that cross out of the
system. Each row is one seam. Its columns are:

- **Seam**: the seam's name, such as `http:stripe/payments`. It starts with
  the kind of seam, then the provider, then what it provides.
- **Kind**: how the two sides talk, such as `http` for a web API, `event`
  for messages one side sends and the other picks up, or `file` for a file
  one side writes and the other reads.
- **Provider** and **Consumers**: the two sides. A side marked `external`
  belongs to someone outside the team, such as a vendor or a customer. One
  marked `in this repo` is built in the same code repository as the
  register. A seam with `no single owner`, such as a database table several
  parts of the system share, has no one provider.
- **Audience**: who can use the seam: `internal`, only the team's own
  system, or `public`, anyone outside it.
- **Contract**: where the written agreement between the two sides lives, and
  who owns it, or `none` when there isn't one yet.
- **Deploys independently**: `yes` when the two sides are released at
  different times, `no` when they're always released together, and `not
  known` when nobody has said yet.
- **Test world**: the tests that check a side against the contract: which
  side each tests, when it runs, and the tool that runs it.
- **Rung**: how far up the ladder the seam is, out of its top. **4 of 6**
  means it has reached rung 4 of a ladder that goes to 6. **5 of 5** means it
  has reached its top. **not rated** means nobody has rated it yet, so its
  protection is unknown.
- **Gaps**: what the seam needs to reach the next rung, the most pressing
  first.

A dash, `—`, means there's nothing to show: a seam with no tests yet has a
dash for its test world.

The **Rung** and **Gaps** columns are there only when the seams have been
rated. Without a rating, the register lists the seams, but not how well each
is protected.

### The history

At the end, the history table shows how the register has changed over time.
Each row is one day's count: how many seams there were, how many were at each
rung, and how many weren't rated. It shows whether protection keeps pace as
the system grows. A dash in a rung's column means that day had no ratings.

## Who rates a seam

The register doesn't judge how well a seam is protected. Another tool, such
as sheepdog, looks at each seam's contract and tests, and decides its rung
and its gaps. The register only shows what that tool decided. So if a rating
looks wrong, it's that tool's judgement to question.

## What to do with it

Start at the top of each table. The register already lists the seams that
most need attention first: those not rated, then those on the lowest rung,
and up from there. For each seam, its gaps say what it needs next.
