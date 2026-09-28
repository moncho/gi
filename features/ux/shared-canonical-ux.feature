@canonical @piclaw-3.2.4
Feature: Piclaw-compatible interaction model
  Installed Piclaw 3.2.4 is the behavioral oracle. The original Tau/Vibes
  contract is preserved under features/ux/upstream/shared-canonical-ux.gherkin.
  Unsupported Gi capabilities remain gaps; deliberate safety deviations are
  documented separately and do not earn Piclaw parity credit.

  Background:
    Given an isolated canonical database
    And the canonical current session "main"
    And a second session "research"
    And deterministic native assets and registry data
    And all actions are performed through visible enabled controls

  @shell @pointer @keyboard
  Scenario Outline: Open and dismiss the workspace menu
    Given the workspace menu is closed
    When I open the workspace menu using <input>
    Then the workspace menu is open exactly once
    And focus can reach each enabled menu item
    When I dismiss the workspace menu using <dismissal>
    Then the workspace menu is closed
    And no underlying control is activated
    And focus returns to a usable shell control

    Examples:
      | input    | dismissal      |
      | pointer  | outside pointer|
      | keyboard | Escape         |

  @shell @workspace @responsive
  Scenario: Show and hide the native workspace
    Given the composer contains canonical unsent content
    When I choose "Show workspace" from the workspace menu
    Then the native workspace tree is visible
    And the workspace menu is closed
    When I hide the workspace
    Then the current session and composer content are unchanged
    And on narrow layouts the drawer backdrop activates no Plan or composer control

  @quick-actions @typeahead @keyboard
  Scenario: Type on the idle timeline to open Quick actions
    Given focus is on noninteractive timeline content
    And no modal, session picker, model picker, workspace editor or composer control is active
    When I type one printable non-whitespace character without Control, Meta or Alt
    Then Quick actions opens exactly once
    And its search field has focus
    And the typed character is the initial query
    And matching sessions, workspace actions and supported slash commands are grouped in native order
    And the highlighted result prefers exact title, then title prefix, then the first result
    When I press ArrowDown or ArrowUp
    Then the highlight wraps through the filtered results
    When I press Enter
    Then the highlighted action runs exactly once
    And Quick actions closes without erasing the composer draft

  @quick-actions @typeahead @focus @failure
  Scenario Outline: Do not steal typing from an interactive surface
    Given focus is inside <surface>
    When I type a printable character
    Then Quick actions remains closed
    And the surface receives the character normally

    Examples:
      | surface                  |
      | composer textarea        |
      | input or select          |
      | button or link           |
      | contenteditable editor   |
      | workspace sidebar        |
      | open modal dialog        |
      | session or model picker  |

  @quick-actions @typeahead @ime
  Scenario: Ignore consumed, modified and composing keys
    Given focus is on noninteractive timeline content
    When a key event is already prevented, repeated, composing, whitespace, Control-modified, Meta-modified or Alt-modified
    Then Quick actions remains closed
    And no action is activated

  @quick-actions @dismissal @scope
  Scenario Outline: Dismiss Quick actions without side effects
    Given Quick actions was opened from a connected visible trigger
    And its search query has not activated an action
    When I dismiss it using <dismissal>
    Then Quick actions is closed
    And focus returns to the connected opening trigger when applicable
    And the current session, composer draft, media and references are unchanged

    Examples:
      | dismissal       |
      | Escape          |
      | outside pointer |
      | close control   |

  @quick-actions @scope @race @failure
  Scenario: Activate only current supported Quick actions
    Given command and session results are scoped to session "main"
    When I change to session "research" while an older catalogue or activation is pending
    Then the older result cannot replace or activate an action in "research"
    And failed activation keeps Quick actions open with recoverable input and an error
    And unsupported commands and workspace actions are absent rather than simulated
    And selecting a slash command replaces the existing composer draft with its exact command text without submitting it

  @quick-actions @skills @commands @scope
  Scenario: Discover loaded skills through canonical slash commands
    Given session "main" has two loaded skills with distinct names and descriptions
    When Quick actions loads the authoritative command catalogue
    Then each loaded skill appears exactly once as "/skill:<name>"
    And skill commands are searchable by name and description in the Slash commands group
    And no separate Skills group or synthetic skill action is added
    When I activate one skill command
    Then the composer replaces its existing draft with exactly "/skill:<name>"
    And it receives focus without submitting a prompt
    # A fixture-listed skill demonstrates UI prefill only. Loading, execution
    # and stale-command errors require an authenticated backend journey.

  @plan @addon-dependent @pointer @keyboard
  Scenario Outline: Open Plan and edit the stored Markdown
    Given the installed Plan sidebar add-on has stored Markdown for session "main"
    When I open Plan using <input>
    Then its editor and checklist progress are visible
    When I edit and Save the Plan
    Then the add-on posts the captured chat identifier and Markdown
    And a successful response updates its saved timestamp
    And a reload loads the saved text for that chat

    Examples:
      | input    |
      | pointer  |
      | keyboard |

  @plan @addon-dependent @race @failure
  Scenario: Preserve dirty Plan text on a remote update until explicit Refresh
    Given the installed Plan add-on editor has unsaved local text
    When a same-chat plan.changes event reports different stored Markdown
    Then the editor retains its local text and warns about the remote change
    When I activate Refresh explicitly
    Then an applicable stored Markdown response replaces the unsaved text
    And no additional discard confirmation is shown
    # The installed add-on uses updated_at and local edit guards, not server CAS.

  @plan @addon-dependent @scope @submit
  Scenario: Submit Plan to the captured chat
    Given the installed Plan add-on editor and composer both contain unsent content
    When I choose "Submit to model"
    Then the add-on saves its Markdown for the captured chat before submission
    And it sends a nonempty saved checklist prompt to that chat in auto mode
    And save failure, empty saved text or a changed selected chat prevents submission
    And the composer draft is not used as the Plan submission text

  @plan @addon-dependent @tool @model @truthful-ui
  Scenario: Expose stored Plan Markdown through the installed add-on tool
    Given session "main" has checklist items in pending, in-progress and completed states
    Then Plan interprets "- [ ]", "- [-]" and "- [x]" as checklist states
    And headings and non-checklist Markdown remain editable
    When the installed "plan" tool reads or updates that chat's Plan
    Then it uses stored Markdown and updated_at rather than a server CAS revision
    And an update notifies the matching sidebar without changing another chat's Plan

  @session-picker @pointer @keyboard
  Scenario Outline: Open, search and dismiss the session picker
    When I open the session picker using <input>
    Then search has focus before the first visible paint
    And the popup remains anchored to its native composer target
    And sessions "main" and "research" are present by native identifier
    When I dismiss it with Escape
    Then no session changes
    And focus returns to the session-picker trigger

    Examples:
      | input    |
      | pointer  |
      | keyboard |

  @session-picker @scope @race
  Scenario: Select one coherent session view
    Given delayed responses exist for session "main"
    When I select session "research" using keyboard navigation
    Then timeline, queue, model, context and composer all show "research"
    And a late "main" response replaces none of them

  @session-picker @capability
  Scenario: Expose only supported session mutations
    Then pin, archive, restore, rename, delete and child-session creation are enabled only when implemented by the native API
    And running or unknown-count sessions cannot be deleted
    And a failed mutation keeps the picker and selection recoverable

  @queue @fifo
  Scenario: Queue two follow-ups exactly once
    Given session "main" has an active turn
    When I send two canonical follow-ups
    Then both native queue IDs are visible in FIFO order
    And their text, media and references are stored once
    And session "research" is unchanged

  @queue @return @race @failure
  Scenario: Return a queued item by replacing the Classic editor draft
    Given the composer contains newer unsent text and media
    And a queued item contains text and serialised references
    When I activate Return to editor for that item
    Then the client replaces the composer text and references with queued content
    And it clears the composer's media list and submission notices
    And it schedules focus at the end of the restored text and row removal
    # A disposable UI fixture confirms text replacement and the removal
    # request; media, failure/retry and backend deletion still need coverage.

  @queue @remove @reorder @scope
  Scenario: Reorder and remove by durable identity
    When I move one queued item by one adjacent position
    Then only that target group's persisted FIFO order changes
    When native removal rejects the selected queue ID
    Then the selected row remains or reconciles to authoritative consumed state
    And no other session or composer draft changes

  @queue @steer @current-behavior
  Scenario: Let the backend steer or send a queued item after the stream ends
    Given the Classic follow-up stack offers Steer for a queued item
    When I activate Steer
    Then the client calls the backend with the row and chat identifiers
    And the backend decides whether to steer an active run or send immediately after it ends
    And a failed request warns and refreshes the queue
    # Exact once-only consumption and idle-run ownership need a backend test.

  @model-picker @pointer @keyboard
  Scenario Outline: Search and select a model authoritatively
    Given two real registry models with explicit capabilities
    And the composer has unsent text and references
    When I open the model picker using <input>
    And I search for and select the second model
    Then the accepted native mutation updates the session model and context window
    And reload preserves the selected model only for session "main"
    And composer content is unchanged

    Examples:
      | input    |
      | pointer  |
      | keyboard |

  @session-picker @model-picker @typeahead @keyboard @capability
  Scenario Outline: Find and activate picker entries without changing unsupported state
    Given the <picker> contains multiple authoritative entries with similar names
    When I search by native identifier, display name or capability metadata
    Then only matching entries remain in native grouped order
    When focus leaves the search field and I type a printable unmodified prefix
    Then incremental typeahead highlights the prefix match before substring matches
    And Arrow keys, Home, End, PageUp and PageDown move within enabled results
    And Enter activates the highlighted enabled entry exactly once
    And Escape closes the picker and restores focus without changing selection
    And unavailable mutations or models remain absent or disabled rather than simulated

    Examples:
      | picker         |
      | session picker |
      | model picker   |

  @model-picker @failure @race @truthful-ui
  Scenario: Reject stale or unsupported model state
    Given a model switch is pending for session "main"
    When I switch to session "research"
    Then the late response cannot change the "research" model label
    And a rejected switch retains the prior model and composer draft
    And thinking appears only for advertised support
    And unknown context remains unavailable
    And local token estimates are labelled estimates
    And compaction is actionable only when natively supported

  @turn @reconnect @scope
  Scenario: Cancel the captured active turn across reconnect
    Given a busy turn has captured session, turn and runtime owner
    When SSE disconnects and reconnects
    Then busy state is refreshed without changing ownership
    When I activate the distinct stop control
    Then only that captured turn is cancelled
    And composer and queue are preserved
    And a stale terminal event cannot stop a newer turn

  @timeline @copy @delete @pointer @keyboard @failure
  Scenario: Copy and delete timeline messages through native actions
    Given the timeline contains user and assistant Markdown with a code block
    Then each deletable message exposes an accessible Delete message action
    And each copyable message exposes an accessible Copy message action
    And each code block exposes its own Copy code action
    When I copy the message or code block
    Then the clipboard receives original stored Markdown or code rather than rendered HTML
    And success or failure glyphs are announced and return to idle after the native timeout
    When native deletion rejects or accepts the captured message ID
    Then only that message remains or is removed according to the authoritative response
    And no other session, message reference or composer draft changes

  @messages @model @range @scope @failure
  Scenario: Let the model identify bounded ranges of persisted messages
    Given session "main" contains ordered persisted messages with durable numeric IDs
    When the model requests multiple explicit message IDs with context before and after
    Then the native messages tool returns them in timeline order with bounded surrounding rows
    And missing IDs are reported without substituting another session's content
    When the model requests an after-row or before-row window with a bounded limit
    Then only messages inside that current-session window are returned
    And content and result counts are bounded and pagination metadata is truthful
    And quoted message content is data rather than new instructions

  @attachments @failure
  Scenario: Retry attachment delivery without duplication
    Given upload or paste shows one native progress control
    When I cancel and retry the selected file
    Then cancellation prevents send and retains the draft
    And retry delivers one durable media item to the active destination
    And it survives reload and source removal

  @tools @pane @glyph @timer @reconnect @accessibility
  Scenario: Present tool execution lifecycle in the native tool pane
    Given the captured turn emits a tool call with a durable tool-call ID and start time
    When I open its tool pane using pointer or keyboard
    Then focus reaches the pane and its disclosure state is announced
    And the running glyph, tool name, arguments and elapsed timer are visible
    And the elapsed label advances from the authoritative start time at the native cadence
    When the matching result succeeds, fails or is cancelled
    Then the corresponding terminal glyph and accessible label replace the running glyph
    And the timer freezes at the authoritative terminal duration
    And stale or duplicate events cannot alter a newer tool call with the same display name
    And reconnect or reload reconstructs the same lifecycle from persisted events
    And closing the pane restores usable focus without activating underlying controls
    And reduced-motion mode preserves state meaning without requiring animation

  @timeline @svg @security @accessibility
  Scenario: Render safe fenced SVG as an isolated image with source fallback
    Given an assistant message contains a fenced "svg" block with safe vector geometry and a title
    When the shipped Classic renderer processes it with sanitization enabled
    Then the timeline displays a data:image/svg+xml image with accessible title
    And source remains available for code copy without privileged inline SVG DOM
    When a fence contains a script, event handler and external image reference
    Then it stays visible as escaped source with no preview image
    And it cannot run script or fetch that external reference
    # Additional hostile categories, malformed and oversized SVG need separate tests.

  @copy @speech @capability
  Scenario: Copy and read assistant content truthfully
    When I copy an assistant code block
    Then the clipboard receives original stored text rather than highlighted HTML
    And read aloud is shown only when the browser and assistant text support it
    And starting another post transfers speech ownership
    And stale completion callbacks do nothing
