Feature: seams keeps a registry of a repo's seams
  Each seam has an id that names it everywhere, one provider, and its
  consumers, so every tool that reads the registry judges the same seams.

  Scenario: A registry with a malformed id is refused, naming the seam
    Given a registry with a seam whose id is quote-accepted
    When a developer runs seams validate
    Then it fails, naming quote-accepted and the form kind:provider/name

  Scenario: A developer registers a seam and finds it in the list
    Given an empty registry
    When a developer adds the seam event:backend/quote-accepted, provided by backend and consumed by billing
    Then seams list shows event:backend/quote-accepted, with backend as its provider and billing as its consumer

  Scenario: A tool reads a seam as JSON
    Given a registry with the seam http:stripe/payments
    When a tool runs seams show http:stripe/payments --json
    Then it gets the seam as JSON matching the published schema

  Scenario: A provider is recorded as not in this repo
    Given an empty registry
    When a developer runs seams add cli:tool/registry --consumer app --not-in-repo tool
    Then the registry records tool on cli:tool/registry as not in this repo
    And seams list --json reports tool on cli:tool/registry as not in this repo

  Scenario: Updating a seam keeps a party recorded as not in this repo
    Given a registry whose seam cli:tool/registry has a provider not in this repo
    When a developer runs seams update cli:tool/registry --audience public
    Then the registry records tool on cli:tool/registry as not in this repo

  Scenario: Adding another seam keeps a party recorded as not in this repo
    Given a registry whose seam cli:tool/registry has a provider not in this repo
    When a developer runs seams add http:backend/quotes --consumer web
    Then the registry records tool on cli:tool/registry as not in this repo

  Scenario: A party can't be both in and not in this repo
    Given a registry whose seam cli:tool/registry has a provider not in this repo
    When a developer runs seams add http:backend/quotes --consumer web --in-repo web --not-in-repo web
    Then it fails, saying web can't be both in this repo and not, and the registry is unchanged

  Scenario: A flag naming a party that isn't on the seam is refused
    Given a registry whose seam cli:tool/registry has a provider not in this repo
    When a developer runs seams add http:backend/quotes --consumer web --not-in-repo nobody
    Then it fails, naming nobody, and the registry is unchanged
    When a developer runs seams update cli:tool/registry --in-repo nobody
    Then it fails, naming nobody, and the registry is unchanged

  Scenario: Update moves a party into or out of this repo
    Given a registry whose seam cli:tool/registry has a provider in this repo
    When a developer runs seams update cli:tool/registry --not-in-repo tool
    Then the registry records tool on cli:tool/registry as not in this repo
    When a developer runs seams update cli:tool/registry --in-repo tool
    Then the registry records tool on cli:tool/registry as in this repo

  Scenario: The list's schema allows a party not in this repo
    Given a registry whose seam cli:tool/registry has a provider not in this repo
    When a tool runs seams list --json
    Then it gets the list as JSON matching the published schema, with tool not in this repo

  Scenario: Moving a contract keeps its owner
    Given a registry whose seam cli:tool/registry has the contract schemas/1.0.0, owned by tool
    When a developer runs seams update cli:tool/registry --contract schemas/1.1.0
    Then the registry records the contract of cli:tool/registry at schemas/1.1.0, owned by tool

  Scenario: Moving a contract with a new owner sets both
    Given a registry whose seam cli:tool/registry has the contract schemas/1.0.0, owned by tool
    When a developer runs seams update cli:tool/registry --contract schemas/1.1.0 --owner app
    Then the registry records the contract of cli:tool/registry at schemas/1.1.0, owned by app

  Scenario: A project's seams files are still in the old folder
    Given a project whose seams registry is in .sheepdog/seams/
    When the developer adds a seam
    Then seams refuses, and says to move the folder to .seams/ and change each file's schema id
    And nothing is written to .seams/

  Scenario: A project has both the old folder and the new
    Given a project with seams files in both .sheepdog/seams/ and .seams/
    When the developer lists its seams
    Then seams refuses, and says to merge the old folder into .seams/ by hand and delete it

  Scenario: A registry carries the old schema id
    Given a project whose .seams/registry.json names the old sheepdog-seams schema
    When the developer validates its seams
    Then seams refuses, and gives the new schema id to use in its place

  Scenario: A project with no seams files yet starts in the new folder
    Given a project with no seams files
    When the developer adds a seam
    Then the registry is written to .seams/registry.json, with the new schema id
