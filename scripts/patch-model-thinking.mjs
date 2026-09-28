// Adapt only the supplied picker at build time; reuse native validated thinking.
export function patchModelThinking(source){
 let s=source;const replace=(from,to)=>{if(s.split(from).length!==2)throw Error('Model thinking adapter anchor changed: '+from.slice(0,90));s=s.replace(from,to)};
 replace('    const modelMutationRef = useRef(false);',`    const [thinkingState, setThinkingState] = useState(null);
    const thinkingFocusRef = useRef(false);
    useLayoutEffect(() => {
        if (switchingModel || !thinkingFocusRef.current) return;
        thinkingFocusRef.current = false;
        const control = modelPopupRef.current?.querySelector('select[aria-label="Thinking level"]');
        if (control && !control.disabled && document.activeElement === document.body && !document.querySelector('.settings-dialog[aria-modal="true"]')) control.focus({preventScroll:true});
    }, [switchingModel, thinkingState]);
    const acceptThinkingState = (payload) => {
        if (payload && typeof payload === 'object') setThinkingState(payload);
    };
    const modelMutationRef = useRef(false);`);
 replace('    const emitModelState = (payload) => {','    const emitModelState = (payload) => {\n        acceptThinkingState(payload);');

 replace('    const handleCycleModel = async () => {',`    const handleSelectThinking = async (event) => {
        const control = event.currentTarget;
        const requested = control.value;
        control.value = thinkingState?.thinking_level || '';
        if (modelMutationRef.current || loadingModels || !thinkingState?.thinking_configurable || thinkingState.current !== activeModel || !thinkingState.thinking_token) return;
        if (requested !== '' && !thinkingState.thinking_levels?.includes(requested)) return;
        if (requested === (thinkingState.thinking_level || '')) return;
        thinkingFocusRef.current = document.activeElement === control;
        modelMutationRef.current = true;
        ++modelRevisionRef.current;
        const mutationToken = onModelMutationStart?.();
        setSwitchingModel(true); setSubmitError('');
        try {
            const state = await selectAgentThinking(currentChatJid, activeModel, requested, thinkingState.thinking_token);
            if (!mountedRef.current) return;
            emitModelState(state);
        } catch (error) {
            if (mountedRef.current) {
                setSubmitError('Thinking selection failed: ' + error.message);
                try {
                    const state = await getAgentModels(currentChatJid);
                    if (mountedRef.current) emitModelState(state);
                } catch { if (mountedRef.current) setThinkingState(null); }
            }
        } finally {
            modelMutationRef.current = false;
            onModelMutationEnd?.(mutationToken);
            if (mountedRef.current) {
                setSwitchingModel(false);
            }
        }
    };

    const handleCycleModel = async () => {`);
 replace(`\${supportsThinking && thinkingLevel && html\`<label class="compose-model-catalogue-thinking" title="Thinking level is read-only in Gi"><span>Thinking</span><select aria-label="Thinking level (read-only)" disabled><option value=\${thinkingLevel}>\${thinkingLevel}</option></select></label>\`}`,`\${supportsThinking && html\`<label class="compose-model-catalogue-thinking"><span>Thinking</span><select aria-label="Thinking level" value=\${thinkingState?.thinking_level || ''} disabled=\${loadingModels || switchingModel || !thinkingState?.thinking_configurable || thinkingState.current !== activeModel || !thinkingState.thinking_token} onChange=\${handleSelectThinking}>
                                        <option value="">Provider default</option>
                                        \${(thinkingState?.thinking_levels || []).map(level => html\`<option value=\${level}>\${level}</option>\`)}
                                    </select></label>\`}`);
 return "import { selectAgentThinking } from '../api.js';\n"+s;
}
