import React, { useState } from 'react';
import { Button } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';

export default function ArtifactPlayer({ item, Image }) {
  return <Artwork key={item.artifact} item={item} Image={Image} />;
}

function Artwork({ item, Image }) {
  const { t } = useTranslation();
  const [playing, setPlaying] = useState(false);
  const [evidenceOpen, setEvidenceOpen] = useState(false);
  if (!item.artifact) return null;
  const animation = item.animation;
  return (
    <figure style={{ margin: 0 }}>
      <Image src={playing && animation ? animation.artifact : item.artifact} />
      {animation && (
        <>
          <figcaption className='iq-classic-row'>
            <Button
              size='small'
              aria-pressed={playing}
              onClick={() => setPlaying(!playing)}
            >
              {playing ? t('停止') : t('播放动画记录')}
            </Button>
            <span className='iq-classic-muted'>
              {t('采样窗口')}：
              {((animation.frames - 1) * animation.step_ms) / 1000}s ·{' '}
              {animation.frames} {t('帧')}
            </span>
          </figcaption>
          <details
            onToggle={(event) => setEvidenceOpen(event.currentTarget.open)}
          >
            <summary>{t('连续帧证据')}</summary>
            {evidenceOpen && <Image src={animation.evidence} />}
            <p className='iq-classic-muted'>
              {t('画面按从左到右、从上到下排列。')}{' '}
              {animation.evidence_ms.map((ms) => `${ms / 1000}s`).join(' · ')}
            </p>
          </details>
        </>
      )}
    </figure>
  );
}
