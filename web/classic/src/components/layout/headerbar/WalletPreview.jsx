/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import React from 'react';
import { Link } from 'react-router-dom';
import { WalletCards } from 'lucide-react';
import { renderQuota } from '../../../helpers';

const WalletPreview = ({ userState, isMobile, navigate, t }) => {
  const user = userState?.user;
  if (!user) return null;

  const balance = renderQuota(
    Number(user.quota || 0) + Number(user.gift_quota || 0),
  );

  return (
    <div
      className={`hidden items-center overflow-hidden rounded-full border border-amber-300 bg-amber-50 shadow-sm transition-colors hover:bg-amber-100 dark:border-amber-700 dark:bg-amber-950/40 dark:hover:bg-amber-900/50 ${isMobile ? '' : 'sm:flex'}`}
      aria-label={t('钱包管理')}
    >
      <Link
        to='/console/topup'
        className='flex h-8 items-center gap-1.5 px-2.5 text-sm font-semibold text-amber-900 transition-colors hover:bg-amber-100 dark:text-amber-100 dark:hover:bg-amber-900/60'
      >
        <WalletCards
          size={16}
          className='shrink-0 text-amber-700 dark:text-amber-300'
        />
        <span className='font-mono tabular-nums'>{balance}</span>
      </Link>
      <button
        type='button'
        onClick={() => navigate('/console/topup?show_recharge=true')}
        className='mr-0.5 flex h-7 items-center rounded-full bg-orange-500 px-2.5 text-xs font-semibold text-white transition-colors hover:bg-orange-600'
      >
        {t('充值')}
      </button>
    </div>
  );
};

export default WalletPreview;
