Feature: seams detect proposes a repo's seams, with evidence
  Detection reads a repo's manifests, code, contracts and infrastructure as
  code, and proposes each seam it finds for a person to confirm. It proposes
  rather than decides, so each candidate says why, and how sure it is.

  Scenario: A developer asks which seams a repo has
    Given the fixture system
    When a developer runs seams detect
    Then each candidate is listed with its kind, provider, consumers, evidence by file and line, and confidence

  Scenario: A developer sees how the registry has drifted from the code
    Given a registry that lacks a seam the code has, and lists one the code no longer has
    When a developer runs seams detect --drift
    Then both are named, each with why
