import { useState, useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { donationAPI } from '../api';
import { Donation } from '../types';

const Profile = () => {
  const { user, logout } = useAuth();
  const [donations, setDonations] = useState<Donation[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'info' | 'donations'>('info');

  useEffect(() => {
    if (user) loadDonations();
  }, [user]);

  const loadDonations = async () => {
    try {
      const response = await donationAPI.getMyDonations({ limit: 20 });
      setDonations(response.data.donations);
    } catch (error) {
      console.error('加载捐赠记录失败:', error);
    } finally {
      setLoading(false);
    }
  };

  const viewCertificate = async (donationId: string) => {
    try {
      const response = await donationAPI.getCertificate(donationId);
      const cert = response.data.certificate;
      alert(`电子凭证\n\n凭证编号：${cert.certificateNo}\n捐赠金额：¥${cert.amount}\n项目：${cert.projectTitle}\n捐赠人：${cert.donorName}\n捐赠时间：${new Date(cert.createdAt).toLocaleString()}`);
    } catch (error: any) {
      alert(error.response?.data?.message || '获取凭证失败');
    }
  };

  const applyRefund = async (donationId: string) => {
    const reason = window.prompt('请填写退款原因（提交后由平台管理员审核）');
    if (reason === null) return;
    if (!reason.trim()) {
      alert('请填写退款原因');
      return;
    }
    try {
      const response = await donationAPI.applyRefund(donationId, { reason: reason.trim() });
      const refund = response.data.refund;
      if (refund?.status === 'pending') {
        alert('退款申请已提交，请等待管理员审核');
      } else {
        alert(`该笔捐赠已有退款申请，当前状态：${refundStatusMap[refund?.status] || refund?.status}`);
      }
      loadDonations();
    } catch (error: any) {
      alert(error.response?.data?.message || '退款申请提交失败');
    }
  };

  if (!user) {
    return <Navigate to="/login" />;
  }

  const roleMap: Record<string, string> = {
    user: '个人用户',
    org: '公益组织',
    admin: '管理员',
  };

  // 退款申请窗口：捐款后两天（48 小时）内可申请，与后端规则一致。
  const REFUND_WINDOW_MS = 48 * 3600 * 1000;
  const withinRefundWindow = (createdAt: string) =>
    Date.now() - new Date(createdAt).getTime() <= REFUND_WINDOW_MS;

  const refundStatusMap: Record<string, string> = {
    pending: '退款审核中',
    approved: '退款已核准',
    rejected: '退款已驳回',
  };
  const refundStatusClass: Record<string, string> = {
    pending: 'bg-yellow-100 text-yellow-700',
    approved: 'bg-red-100 text-red-600',
    rejected: 'bg-gray-200 text-gray-500',
  };

  return (
    <div className="max-w-4xl mx-auto">
      <div className="bg-white rounded-2xl shadow-sm overflow-hidden mb-6">
        <div className="bg-gradient-to-r from-primary-500 to-primary-600 h-32" />
        <div className="px-8 pb-8">
          <div className="flex items-end gap-6 -mt-12 mb-6">
            <div className="w-24 h-24 bg-white rounded-full border-4 border-white shadow-lg flex items-center justify-center">
              <span className="text-3xl font-bold text-primary-600">
                {(user.realName || user.username).charAt(0)}
              </span>
            </div>
            <div className="mb-2">
              <h1 className="text-2xl font-bold text-gray-900">{user.realName || user.username}</h1>
              <p className="text-gray-500">@{user.username}</p>
            </div>
            <span className="mb-3 px-3 py-1 bg-primary-100 text-primary-700 rounded-full text-sm font-medium">
              {roleMap[user.role]}
            </span>
          </div>

          <div className="grid grid-cols-3 gap-8">
            <div className="text-center p-4 bg-gray-50 rounded-xl">
              <div className="text-3xl font-bold text-primary-600">¥{user.totalDonation?.toLocaleString() || 0}</div>
              <div className="text-sm text-gray-500 mt-1">累计捐赠</div>
            </div>
            <div className="text-center p-4 bg-gray-50 rounded-xl">
              <div className="text-3xl font-bold text-green-600">{user.serviceHours || 0}</div>
              <div className="text-sm text-gray-500 mt-1">服务时长</div>
            </div>
            <div className="text-center p-4 bg-gray-50 rounded-xl">
              <div className="text-3xl font-bold text-blue-600">{donations.length}</div>
              <div className="text-sm text-gray-500 mt-1">捐赠次数</div>
            </div>
          </div>
        </div>
      </div>

      <div className="bg-white rounded-2xl shadow-sm">
        <div className="border-b border-gray-100">
          <div className="flex gap-8 px-8">
            <button
              onClick={() => setActiveTab('info')}
              className={`py-4 font-medium ${activeTab === 'info' ? 'text-primary-600 border-b-2 border-primary-600' : 'text-gray-500'}`}
            >
              个人信息
            </button>
            <button
              onClick={() => setActiveTab('donations')}
              className={`py-4 font-medium ${activeTab === 'donations' ? 'text-primary-600 border-b-2 border-primary-600' : 'text-gray-500'}`}
            >
              捐赠记录
            </button>
          </div>
        </div>

        <div className="p-8">
          {activeTab === 'info' && (
            <div className="max-w-lg">
              <div className="space-y-6">
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">用户名</label>
                  <div className="text-gray-900">{user.username}</div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">邮箱</label>
                  <div className="text-gray-900">{user.email}</div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">真实姓名</label>
                  <div className="text-gray-900">{user.realName || '-'}</div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">手机号</label>
                  <div className="text-gray-900">{user.phone || '-'}</div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-500 mb-1">注册时间</label>
                  <div className="text-gray-900">{user.createdAt ? new Date(user.createdAt).toLocaleDateString() : '-'}</div>
                </div>
              </div>
              <button
                onClick={logout}
                className="mt-8 px-6 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100"
              >
                退出登录
              </button>
            </div>
          )}

          {activeTab === 'donations' && (
            <div>
              {loading ? (
                <div className="text-center py-8">加载中...</div>
              ) : donations.length === 0 ? (
                <div className="text-center py-12 text-gray-500">
                  暂无捐赠记录
                </div>
              ) : (
                <div className="space-y-4">
                  {donations.map((donation) => (
                    <div key={donation.id} className="flex items-center justify-between p-4 bg-gray-50 rounded-xl">
                      <div className="flex-1">
                        <div className="flex items-center gap-2">
                          <div className="font-medium text-gray-900">{donation.project?.title}</div>
                          {donation.paymentStatus === 'refunded' && (
                            <span className="px-2 py-0.5 bg-red-100 text-red-600 rounded text-xs font-medium">
                              已退款
                            </span>
                          )}
                          {donation.paymentStatus !== 'refunded' && donation.refund && (
                            <span className={`px-2 py-0.5 rounded text-xs font-medium ${refundStatusClass[donation.refund.status] || 'bg-gray-200 text-gray-500'}`}>
                              {refundStatusMap[donation.refund.status] || donation.refund.status}
                            </span>
                          )}
                        </div>
                        <div className="text-sm text-gray-500">
                          {new Date(donation.createdAt).toLocaleString()}
                        </div>
                        {donation.certificateNo && (
                          <div className="text-sm text-primary-600">
                            凭证号：{donation.certificateNo}
                            {donation.paymentStatus === 'refunded' && (
                              <span className="ml-2 text-gray-400">（已失效）</span>
                            )}
                          </div>
                        )}
                        {donation.refund && (
                          <div className="text-sm text-gray-500 mt-1">
                            退款原因：{donation.refund.reason}
                            {donation.refund.status === 'rejected' && donation.refund.reviewNote && (
                              <span className="ml-2">驳回说明：{donation.refund.reviewNote}</span>
                            )}
                          </div>
                        )}
                      </div>
                      <div className="text-right">
                        <div className={`text-xl font-bold ${donation.paymentStatus === 'refunded' ? 'text-gray-400 line-through' : 'text-primary-600'}`}>
                          ¥{donation.amount.toLocaleString()}
                        </div>
                        <div className="space-x-3">
                          {donation.paymentStatus !== 'refunded' && (
                            <button
                              onClick={() => viewCertificate(donation.id)}
                              className="text-sm text-primary-600 hover:text-primary-700"
                            >
                              查看凭证
                            </button>
                          )}
                          {donation.paymentStatus === 'success' && !donation.refund && withinRefundWindow(donation.createdAt) && (
                            <button
                              onClick={() => applyRefund(donation.id)}
                              className="text-sm text-red-500 hover:text-red-600"
                            >
                              申请退款
                            </button>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default Profile;
