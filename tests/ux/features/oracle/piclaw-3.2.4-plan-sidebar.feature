@oracle-only @piclaw-3.2.4 @addon-plan-0.1.25 @source-reviewed @not-gi-parity
Feature: Plan sidebar behaviour in the installed Piclaw add-on
  These clauses describe @rcarmo/piclaw-addon-plan-sidebar 0.1.25 shipped with
  the audited Piclaw workspace. They are source-backed, not browser-executed.
  They neither grant Gi Plan parity nor revise the frozen Classic corpus.

  Background:
    Given the Plan sidebar add-on is installed for the selected chat
    And that chat's Plan is stored as Markdown with an updated_at timestamp

  @oracle-plan-save
  Scenario: Save the captured chat's Plan without clearing newer local edits
    Given the Plan editor contains locally changed Markdown
    When I activate Save
    Then the add-on posts the captured chat identifier and editor Markdown to its Plan API
    And a successful response updates the saved timestamp
    And the dirty flag clears only if the editor still equals the submitted Markdown
    And edits made after the request remain visibly unsaved

  @oracle-plan-remote @conflicts-shared-20
  Scenario: A remote Plan update warns without replacing dirty text
    Given the Plan editor contains unsaved local changes
    When a plan.changes update for the selected chat arrives
    Then the add-on retains the local text and warns that the Plan changed remotely
    When I activate Refresh explicitly
    Then the add-on loads stored Markdown without automatic dirty preservation
    And applicable remote Markdown replaces the editor text
    But no additional discard confirmation is required by this installed add-on

  @oracle-plan-submit
  Scenario: Submit a nonempty saved Plan to the captured chat
    Given the Plan editor contains Markdown and the sidebar is open
    When I activate Submit to model
    Then the add-on first saves the editor Markdown for the captured chat
    And it does not submit if saving fails, the selected chat changes or the saved Plan is empty
    And otherwise it sends a checklist prompt to the captured chat in auto mode
    And a submission error is shown in the sidebar for the still-selected chat

  @oracle-plan-tool @conflicts-shared-revision
  Scenario: Read and update stored checklist Markdown through the Plan tool
    Given the selected chat has pending, in-progress and completed checklist items
    When the model reads Plan using the plan tool
    Then it receives the selected chat's Markdown, updated_at and parsed items
    When the model updates Plan using a supported plan action
    Then the stored Markdown and updated_at change for the targeted chat
    And the sidebar receives a plan.changes notification
    But this add-on does not promise a server-side revision compare-and-swap token
