@gi-specific
Feature: Gi behaviour retained where installed Piclaw 3.2.4 differs
  # These clauses describe Gi's tested implementation. They do not award Classic or Shared parity.
  # Oracle corrections and evidence limits: docs/internal/ux-reaudit-2026-09-26/gherkin-alignment.md

  @gi-ux-001
  Scenario: Slash Quick Actions insert a trailing space without submitting
    Given an available slash Quick Action
    When I select /model or /compact from the Quick Actions palette
    Then Gi prefills the composer with the command and a trailing space
    And it focuses the composer without submitting a turn

  @gi-ux-002
  Scenario: Loaded skill selection retains the draft and media for Gi execution
    Given a loaded /skill:proof Quick Action, draft text and a media attachment
    When I select the skill command from the Quick Actions palette
    Then Gi prepends the command to the existing draft and keeps the attachment
    And it does not submit until I explicitly send the composer
    When I send while switching to another session
    Then the captured session receives one turn with the loaded skill and media
    And stale or unknown skill commands retain recoverable text with an error

  @gi-ux-003
  Scenario: Gi leaves fenced SVG as copyable source instead of an image preview
    Given an assistant message containing a fenced SVG block
    When Gi renders the message
    Then the SVG remains escaped source code and no image preview appears
    And the source can still be copied

  # Piclaw replacement is now implemented; async recovery/persistence remains
  # native-specific and is not a blanket queue parity mapping.
  @gi-ux-004 @migration-gap
  Scenario: Queue return replaces the draft with guarded durable recovery
    Given a queued item and an editor draft with references or media
    When I return the queued item to Gi's editor
    Then queued text and references replace the existing draft
    And edits during attachment recovery prevent replacement and removal until explicit retry
    And recovery is persisted before the queued row is removed
    And failure or reload retains a recoverable item without duplicate recovery
    And typing after replacement survives delayed removal responses

  # Bounded regression only; full Piclaw active/end-race parity remains open.
  # Explicit observed-idle actions now send the selected queued item.
  @gi-ux-005 @migration-gap
  Scenario: Gi Steer fences observed active or idle admission
    Given a queued item associated with a session and an active run
    When I steer it in Gi
    Then stale or foreign run ownership is rejected without consuming the item
    And a matching active run accepts it once
    And a failed request leaves the item available for an explicit retry
    And an explicitly idle action sends the selected queued item once
    And an existing Stop hold continues protecting other queued items
