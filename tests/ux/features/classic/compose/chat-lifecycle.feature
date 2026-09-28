@classic @piclaw-3.2.4 @chat-lifecycle
Feature: Separate the conversation from transient agent activity
  # Oracle: installed Piclaw 3.2.4 event translator and shipped Classic UI.
  # tests/ux/oracle/piclaw-chat-lifecycle-probe.mjs records the exact source,
  # emitted frames and rendered DOM. This is not a Gi implementation fixture.
  # The retained Gi manual-validation baseline fails these contracts.
  # Conversation/idle fixes have bounded native coverage; none is fully mapped.
  # Error persistence, provider recovery and physical-device acceptance need
  # backend and device evidence beyond this bounded rendering probe.

  @ux-chat-lifecycle-001 @idle
  Scenario: An idle chat does not manufacture an activity pane
    Given the selected chat has an authoritative idle snapshot with no active status data
    And there are no transient previews, requests or extension panels
    When the Classic chat opens or reloads
    Then the conversation and composer remain visible
    And there is no agent activity pane saying Idle or Working
    And a previous tool completion does not become a persistent Completed footer

  @ux-chat-lifecycle-002 @streaming @thoughts @draft
  Scenario: Streaming thoughts and response drafts have separate panes
    Given a selected chat contains the user's submitted prompt
    When its current turn streams thinking text and assistant response text
    Then the Thoughts pane contains the thinking text
    And the Draft pane contains the response text
    And the current phase says Writing response
    And these previews do not become extra user messages in the timeline
    And internal tool-call markers are not inserted into the response text

  @ux-chat-lifecycle-003 @tools @output
  Scenario: Tool output belongs to the Output status pane
    Given a tool starts with a call identity and arguments in the selected turn
    When a tool execution update supplies text output
    Then the Output pane displays the supplied preview using Classic's tool-output renderer
    And the activity status identifies the tool while the Output pane is shown
    And raw tool-result records do not appear as conversation posts
    And tool output is never attributed to the user
    # Preserve Classic's renderer, including its output trimming and Markdown
    # rules; this does not require plain preformatted text or identical bytes.

  @ux-chat-lifecycle-004 @author @markdown
  Scenario: A persisted assistant reply retains its identity and Markdown
    Given the conversation contains a user prompt and a persisted assistant response
    When the timeline renders the messages
    Then the user prompt uses the user name and avatar
    And the assistant response uses the assistant name and avatar
    And the assistant response has agent-post presentation
    And Markdown emphasis and list items render as emphasis and list items
    And the persisted assistant response is not attributed to the user

  @ux-chat-lifecycle-005 @error
  Scenario: A terminal provider error is not a user input or a tool success
    Given the current turn has streamed previews and completed a tool
    When the provider fails and the agent emits a terminal error status
    Then the error has an agent error presentation rather than a user post
    And the current turn's draft and thought previews are cleared
    And there is no Completed tool footer standing in for the failed turn
    When the chat reloads with an authoritative idle snapshot
    Then it does not recreate an active or completed tool pane
    # This scenario does not prescribe how a durable error notice is stored.
    # The probe verifies transient status and idle reload, not error persistence.
