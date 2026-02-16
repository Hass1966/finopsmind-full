import React, { useState } from 'react';
import TerraformViewer from './TerraformViewer';

interface Recommendation {
  id: string;
  type: string;
  resourceId?: string;
  resourceName?: string;
  monthlySavings?: number;
}

interface TerraformButtonProps {
  recommendation: Recommendation;
}

const SUPPORTED_TYPES = [
  'ec2_rightsize', 'ec2_stop', 'ec2_terminate',
  'ebs_rightsize', 'ebs_gp3_upgrade', 'ebs_delete',
  's3_lifecycle', 's3_intelligent_tiering',
  'rds_rightsize', 'rds_stop', 'lambda_memory',
  'vpc_endpoint_s3', 'vpc_endpoint_dynamodb',
  'eip_release', 'snapshot_delete', 'cloudwatch_retention'
];

export const TerraformButton: React.FC<TerraformButtonProps> = ({ recommendation }) => {
  const [showViewer, setShowViewer] = useState(false);

  const isSupported = SUPPORTED_TYPES.some(t => 
    recommendation.type.toLowerCase().includes(t) || t.includes(recommendation.type.toLowerCase())
  );

  if (!isSupported) return null;

  return (
    <>
      <button
        onClick={() => setShowViewer(true)}
        style={{
          padding: '6px 12px',
          background: '#7b61ff',
          color: 'white',
          border: 'none',
          borderRadius: '4px',
          fontSize: '12px',
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
        }}
      >
        ⬡ View Terraform
      </button>

      {showViewer && (
        <div
          style={{
            position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
            background: 'rgba(0, 0, 0, 0.75)',
            display: 'flex', justifyContent: 'center', alignItems: 'center',
            zIndex: 1000, padding: '20px',
          }}
          onClick={() => setShowViewer(false)}
        >
          <div style={{ width: '100%', maxWidth: '900px', maxHeight: '90vh', overflow: 'auto' }} onClick={(e) => e.stopPropagation()}>
            <TerraformViewer recommendationId={recommendation.id} onClose={() => setShowViewer(false)} />
          </div>
        </div>
      )}
    </>
  );
};

export default TerraformButton;
