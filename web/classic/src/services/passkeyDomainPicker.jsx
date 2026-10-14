import React from 'react';
import { createRoot } from 'react-dom/client';
import { Modal, Button } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

function DomainPicker({ domains, done }) {
  const { t } = useTranslation();
  return (
    <Modal
      visible
      title={t('选择 Passkey 域名')}
      footer={null}
      onCancel={() => done()}
    >
      <div style={{ display: 'grid', gap: 8 }}>
        {domains.map((domain) => (
          <Button key={domain} onClick={() => done(domain)}>
            {domain}
          </Button>
        ))}
      </div>
    </Modal>
  );
}
export function choosePasskeyDomain(domains) {
  if (!domains || domains.length < 2) return Promise.resolve(domains?.[0]);
  return new Promise((resolve, reject) => {
    const host = document.createElement('div');
    document.body.appendChild(host);
    const root = createRoot(host);
    const done = (domain) => {
      root.unmount();
      host.remove();
      if (domain) resolve(domain);
      else reject(new DOMException('Cancelled', 'AbortError'));
    };
    root.render(<DomainPicker domains={domains} done={done} />);
  });
}
