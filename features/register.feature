Feature: seams draws the register of a repo's seams
  The team sees every seam the system has, internal and external, with each
  one's characteristics and how well it's covered, in one place that stays up
  to date, so good practice reaches every seam, not only the familiar ones.

  Scenario: A team reads its register, gaps first
    Given a registry of three seams, and a status file that puts one of them at no coverage
    When a developer runs seams show --status status.json
    Then the uncovered seam is listed first, and each seam shows its rung

  Scenario: The register is written for the team to read
    Given a registry with internal seams and one that crosses out of the system
    When a developer runs seams render
    Then docs/SEAMS.md lists the internal seams apart from the external one, under a diagram of the components and their seams
