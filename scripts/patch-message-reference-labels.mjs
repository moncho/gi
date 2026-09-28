// Display-only adaptation; canonical refs and all callbacks stay verbatim.
export function patchMessageReferenceLabels(source) {
 const edits=[
  ["export function QueuedFollowupStack({\n    items = [],", "export function QueuedFollowupStack({\n    messageReferenceLabels = {},\n    items = [],"],
  ["    messageRefs = [],\n    onRemoveMessageRef,", "    messageRefs = [],\n    messageReferenceLabels = {},\n    onRemoveMessageRef,"],
  ["<${QueuedFollowupStack}\n                    items=", "<${QueuedFollowupStack}\n                    messageReferenceLabels=${messageReferenceLabels}\n                    items="],
 ];
 for(const [from,to]of edits){if(source.split(from).length!==2)throw Error('Message reference adapter anchor changed: '+from);source=source.replace(from,to);}
 const label="label=${'msg:' + id}";
 if(source.split(label).length!==3)throw Error('Message reference label anchors changed');
 return "import { messageReferenceLabel } from '../gi-message-reference-label.js';\n"+source.replaceAll(label,"label=${'msg:' + messageReferenceLabel(id, messageReferenceLabels)}");
}
