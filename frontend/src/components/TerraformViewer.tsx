import React, { useState, useEffect, useCallback } from 'react';

interface ValidationResult {
  valid: boolean;
  errors?: Array<{ line?: number; message: string }>;
  warnings?: Array<{ line?: number; message: string }>;
  formatted?: string;
}

interface TerraformMetadata {
  recommendation_type: string;
  template_name: string;
  resource_id: string;
  import_command?: string;
  apply_warnings?: string[];
}

interface TerraformResponse {
  success: boolean;
  hcl?: string;
  formatted?: string;
  validation?: ValidationResult;
  error?: string;
  metadata: TerraformMetadata;
}

interface TerraformViewerProps {
  recommendationId: string;
  onClose?: () => void;
  apiBaseUrl?: string;
}

const highlightHCL = (code: string): string => {
  let h = code
    .replace(/\b(resource|data|variable|output|module|provider|terraform|locals|for_each|count|depends_on|lifecycle|provisioner)\b/g, '<span class="keyword">$1</span>')
    .replace(/"([^"\\]|\\.)*"/g, '<span class="string">$&</span>')
    .replace(/(#.*$|\/\/.*$)/gm, '<span class="comment">$1</span>')
    .replace(/\b(\d+\.?\d*)\b/g, '<span class="number">$1</span>')
    .replace(/\b(true|false|null)\b/g, '<span class="boolean">$1</span>')
    .replace(/(\w+)(\s*=)/g, '<span class="attribute">$1</span>$2');
  return h;
};

const TerraformViewer: React.FC<TerraformViewerProps> = ({ recommendationId, onClose, apiBaseUrl = 'http://localhost:8080' }) => {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<TerraformResponse | null>(null);
  const [copied, setCopied] = useState(false);
  const [showFormatted, setShowFormatted] = useState(true);

  const fetchTerraform = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch(`${apiBaseUrl}/api/v1/recommendations/${recommendationId}/terraform`);
      const result: TerraformResponse = await response.json();
      if (!result.success) setError(result.error || 'Failed to generate');
      else setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch');
    } finally {
      setLoading(false);
    }
  }, [recommendationId, apiBaseUrl]);

  useEffect(() => { fetchTerraform(); }, [fetchTerraform]);

  const handleCopy = async () => {
    if (!data) return;
    const code = showFormatted && data.formatted ? data.formatted : data.hcl;
    if (!code) return;
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) { console.error('Copy failed:', err); }
  };

  const handleDownload = () => {
    window.open(`${apiBaseUrl}/api/v1/recommendations/${recommendationId}/terraform/download`, '_blank');
  };

  const displayCode = showFormatted && data?.formatted ? data.formatted : data?.hcl || '';
  const codeWithLines = displayCode.split('\n').map((line, i) => {
    const num = (i + 1).toString().padStart(3, ' ');
    return `<span class="line-number">${num}</span> ${highlightHCL(line)}`;
  }).join('\n');

  return (
    <div style={{ fontFamily: 'system-ui', background: '#1e1e1e', borderRadius: '8px', overflow: 'hidden', boxShadow: '0 4px 6px rgba(0,0,0,0.3)' }}>
      <style>{`
        .tf-code .keyword { color: #569cd6; }
        .tf-code .string { color: #ce9178; }
        .tf-code .comment { color: #6a9955; font-style: italic; }
        .tf-code .number { color: #b5cea8; }
        .tf-code .boolean { color: #569cd6; }
        .tf-code .attribute { color: #9cdcfe; }
        .tf-code .line-number { color: #858585; user-select: none; margin-right: 16px; }
      `}</style>

      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 16px', background: '#252526', borderBottom: '1px solid #3c3c3c' }}>
        <div style={{ color: '#ccc', fontSize: '14px', fontWeight: 500, display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span style={{ color: '#7b61ff' }}>⬡</span> Terraform Configuration
        </div>
        {onClose && <button onClick={onClose} style={{ padding: '6px 12px', borderRadius: '4px', border: 'none', background: '#3c3c3c', color: '#ccc', cursor: 'pointer' }}>Close</button>}
      </div>

      {loading && <div style={{ padding: '40px', textAlign: 'center', color: '#9cdcfe' }}><p>Generating Terraform code...</p></div>}
      {error && <div style={{ padding: '40px', textAlign: 'center', color: '#f14c4c' }}><p>⚠️ {error}</p><button onClick={fetchTerraform} style={{ padding: '6px 12px', background: '#0e639c', color: 'white', border: 'none', borderRadius: '4px', cursor: 'pointer' }}>Retry</button></div>}

      {data && !loading && !error && (
        <>
          {/* Toolbar */}
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '8px 16px', background: '#2d2d2d', borderBottom: '1px solid #3c3c3c' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#9cdcfe', fontSize: '12px' }}>
              <input type="checkbox" checked={showFormatted} onChange={(e) => setShowFormatted(e.target.checked)} />
              Show formatted
            </label>
            <div style={{ display: 'flex', gap: '8px' }}>
              {data.validation && <span style={{ fontSize: '12px', color: data.validation.valid ? '#4ec9b0' : '#f14c4c' }}>{data.validation.valid ? '✓ Valid' : '✗ Invalid'}</span>}
              <button onClick={handleCopy} style={{ padding: '6px 12px', borderRadius: '4px', border: 'none', background: copied ? '#28a745' : '#3c3c3c', color: copied ? 'white' : '#ccc', cursor: 'pointer', fontSize: '12px' }}>{copied ? '✓ Copied!' : '📋 Copy'}</button>
              <button onClick={handleDownload} style={{ padding: '6px 12px', borderRadius: '4px', border: 'none', background: '#0e639c', color: 'white', cursor: 'pointer', fontSize: '12px' }}>⬇️ Download .tf</button>
            </div>
          </div>

          {/* Code */}
          <div style={{ maxHeight: '500px', overflow: 'auto' }}>
            <pre className="tf-code" style={{ margin: 0, padding: '16px', fontFamily: "'Fira Code', Consolas, monospace", fontSize: '13px', lineHeight: 1.5, color: '#d4d4d4', background: '#1e1e1e', whiteSpace: 'pre', overflowX: 'auto' }} dangerouslySetInnerHTML={{ __html: codeWithLines }} />
          </div>

          {/* Metadata */}
          <div style={{ padding: '12px 16px', background: '#252526', borderTop: '1px solid #3c3c3c' }}>
            <div style={{ color: '#9cdcfe', fontSize: '12px', fontWeight: 500, marginBottom: '8px' }}>Configuration Details</div>
            <div style={{ fontSize: '12px', color: '#ccc' }}>
              <div><span style={{ color: '#858585', marginRight: '8px' }}>Type:</span><span style={{ color: '#4ec9b0', fontFamily: 'monospace' }}>{data.metadata.recommendation_type}</span></div>
              <div><span style={{ color: '#858585', marginRight: '8px' }}>Resource:</span><span style={{ color: '#4ec9b0', fontFamily: 'monospace' }}>{data.metadata.resource_id}</span></div>
              {data.metadata.import_command && <div><span style={{ color: '#858585', marginRight: '8px' }}>Import:</span><span style={{ color: '#4ec9b0', fontFamily: 'monospace' }}>{data.metadata.import_command}</span></div>}
            </div>
            {data.metadata.apply_warnings && data.metadata.apply_warnings.length > 0 && (
              <div style={{ marginTop: '12px', padding: '8px 12px', background: 'rgba(255,200,0,0.1)', borderLeft: '3px solid #cca700', borderRadius: '0 4px 4px 0' }}>
                <div style={{ color: '#cca700', fontSize: '12px', fontWeight: 500 }}>⚠️ Warnings</div>
                {data.metadata.apply_warnings.map((w, i) => <div key={i} style={{ color: '#e8c76b', fontSize: '11px', marginLeft: '8px' }}>• {w}</div>)}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
};

export default TerraformViewer;
