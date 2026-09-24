import React, { useState } from 'react';
import { Donation } from '../types';
import { donationAPI } from '../api';

interface RefundModalProps {
  donation: Donation;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const RefundModal: React.FC<RefundModalProps> = ({ donation, isOpen, onClose, onSuccess }) => {
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState(false);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (reason.trim().length < 2) {
      alert('请填写退款原因（至少 2 个字）');
      return;
    }
    setLoading(true);
    try {
      const res = await donationAPI.applyRefund(donation.id, { reason: reason.trim() });
      alert(res.data?.message || '退款申请已提交');
      setReason('');
      onSuccess();
      onClose();
    } catch (error: any) {
      alert(error.response?.data?.message || '提交失败，请重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-2xl p-8 max-w-md w-full mx-4">
        <div className="flex justify-between items-center mb-6">
          <h2 className="text-2xl font-bold text-gray-900">申请退款</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="mb-6 p-4 bg-gray-50 rounded-lg space-y-1">
          <p className="text-sm text-gray-600">项目：<span className="text-gray-900 font-medium">{donation.project?.title}</span></p>
          <p className="text-sm text-gray-600">金额：<span className="text-primary-600 font-semibold">¥{donation.amount.toLocaleString()}</span></p>
          <p className="text-xs text-gray-400">请在捐赠完成后 48 小时内提交，提交后等待管理员审核</p>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="mb-6">
            <label className="block text-sm font-medium text-gray-700 mb-2">退款原因</label>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="请说明退款原因，如误操作、重复捐款等..."
              className="w-full px-4 py-3 border border-gray-200 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent resize-none"
              rows={4}
              maxLength={500}
            />
            <div className="text-right text-xs text-gray-400 mt-1">{reason.length}/500</div>
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-primary-600 text-white py-3 rounded-lg font-semibold hover:bg-primary-700 disabled:opacity-50"
          >
            {loading ? '提交中...' : '确认提交申请'}
          </button>
        </form>
      </div>
    </div>
  );
};

export default RefundModal;
