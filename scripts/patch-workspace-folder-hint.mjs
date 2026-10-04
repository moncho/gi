// Piclaw 3.2.5 explorer: each folder row offers "Add folder hint for <path>",
// which hands the folder to the composer (onFolderSelect). Backported to the
// supplied 3.2.4 explorer without editing its source bytes.
export function patchWorkspaceFolderHint(source) {
    const edits = [
        ['export function WorkspaceExplorer({\n    onFileSelect,\n', 'export function WorkspaceExplorer({\n    onFileSelect,\n    onFolderSelect,\n'],
        ['    const onFileSelectRef = useRef(onFileSelect);\n', '    const onFileSelectRef = useRef(onFileSelect);\n    const onFolderSelectRef = useRef(onFolderSelect);\n'],
        ['    onFileSelectRef.current = onFileSelect;\n', '    onFileSelectRef.current = onFileSelect;\n    onFolderSelectRef.current = onFolderSelect;\n'],
        ["        const target = event?.currentTarget?.dataset?.uploadTarget || '.';\n        uploadTargetRef.current = target;\n        uploadInputRef.current?.click();\n    }, [uploading]);\n",
         "        const target = event?.currentTarget?.dataset?.uploadTarget || '.';\n        uploadTargetRef.current = target;\n        uploadInputRef.current?.click();\n    }, [uploading]);\n\n" +
         "    const handleFolderHintClick = useCallback((event) => {\n        event?.preventDefault?.();\n        event?.stopPropagation?.();\n" +
         "        const target = event?.currentTarget?.dataset?.folderHintTarget;\n        if (!target) return;\n" +
         "        onFolderSelectRef.current?.(target, nodeMapRef.current.get(target));\n    }, []);\n"],
        ['                                    ${isDir && html`\n                                        <button\n                                            class="workspace-folder-upload"\n                                            data-upload-target=${node.path}',
         '                                    ${isDir && typeof onFolderSelect === \'function\' && html`\n' +
         '                                        <button\n                                            class="workspace-folder-upload"\n' +
         '                                            data-folder-hint-target=${node.path}\n' +
         '                                            title="Add folder hint to compose"\n' +
         '                                            aria-label=${`Add folder hint for ${node.path}`}\n' +
         '                                            onClick=${handleFolderHintClick}\n                                        >\n' +
         '                                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">\n' +
         '                                                <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><path d="M12 11v6"/><path d="M9 14h6"/>\n' +
         '                                            </svg>\n                                        </button>\n                                    `}\n' +
         '                                    ${isDir && html`\n                                        <button\n                                            class="workspace-folder-upload"\n                                            data-upload-target=${node.path}'],
    ];
    for (const [from, to] of edits) {
        if (source.split(from).length !== 2) throw new Error(`Workspace folder-hint anchor changed: ${from.slice(0, 60)}`);
        source = source.replace(from, to);
    }
    return source;
}
