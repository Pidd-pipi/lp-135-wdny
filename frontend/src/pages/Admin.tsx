import { useState, useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { adminAPI, refundAPI } from '../api';
import { Project, Organization, Refund } from '../types';

type Tab = 'projects' | 'organizations' | 'refunds';

const Admin = () => {
  const { user } = useAuth();
  const [activeTab, setActiveTab] = useState<Tab>('projects');
  const [pendingProjects, setPendingProjects] = useState<Project[]>([]);
  const [pendingOrgs, setPendingOrgs] = useState<Organization[]>([]);
  const [pendingRefunds, setPendingRefunds] = useState<Refund[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (user?.role === 'admin') {
      loadPendingItems();
    }
  }, [user]);

  const loadPendingItems = async () => {
    try {
      const [projectsRes, orgsRes, refundsRes] = await Promise.all([
        adminAPI.getPendingProjects(),
        adminAPI.getPendingOrganizations(),
        refundAPI.getPendingRefunds(),
      ]);
      setPendingProjects(projectsRes.data.projects);
      setPendingOrgs(orgsRes.data.organizations);
      setPendingRefunds(refundsRes.data.refunds);
    } catch (error) {
      console.error('加载待审核数据失败:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleReviewProject = async (projectId: string, status: 'approved' | 'rejected') => {
    try {
      await adminAPI.reviewProject(projectId, { status, comment: '' });
      alert(`项目已${status === 'approved' ? '通过' : '拒绝'}`);
      loadPendingItems();
    } catch (error) {
      alert('操作失败');
    }
  };

  const handleReviewOrg = async (orgId: string, status: 'approved' | 'rejected') => {
    try {
      await adminAPI.reviewOrganization(orgId, { status, comment: '' });
      alert(`组织已${status === 'approved' ? '通过' : '拒绝'}`);
      loadPendingItems();
    } catch (error) {
      alert('操作失败');
    }
  };

  const handleReviewRefund = async (refundId: string, status: 'approved' | 'rejected') => {
    const tip = status === 'approved'
      ? '核准后将从项目进度、个人累计与排行榜中扣回该款项，电子凭证随即失效。确认核准？'
      : '确认驳回该退款申请？';
    if (!window.confirm(tip)) return;
    try {
      const res = await refundAPI.reviewRefund(refundId, { status });
      alert(res.data?.message || '操作完成');
      loadPendingItems();
    } catch (error: any) {
      alert(error.response?.data?.message || '操作失败');
    }
  };

  if (!user || user.role !== 'admin') {
    return <Navigate to="/" />;
  }

  const categoryMap: Record<string, string> = {
    education: '助学',
    elderly: '助老',
    medical: '医疗',
    disaster: '救灾',
    environment: '环保',
    other: '其他',
  };

  const tabClass = (tab: Tab) =>
    `px-6 py-2 rounded-lg font-medium ${
      activeTab === tab ? 'bg-primary-600 text-white' : 'bg-white text-gray-600 border border-gray-200'
    }`;

  return (
    <div>
      <h1 className="text-3xl font-bold text-gray-900 mb-8">管理后台</h1>

      <div className="flex gap-4 mb-8">
        <button onClick={() => setActiveTab('projects')} className={tabClass('projects')}>
          待审核项目 ({pendingProjects.length})
        </button>
        <button onClick={() => setActiveTab('organizations')} className={tabClass('organizations')}>
          待审核组织 ({pendingOrgs.length})
        </button>
        <button onClick={() => setActiveTab('refunds')} className={tabClass('refunds')}>
          待处理退款 ({pendingRefunds.length})
        </button>
      </div>

      {loading ? (
        <div className="text-center py-20">加载中...</div>
      ) : activeTab === 'projects' ? (
        <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
          {pendingProjects.length === 0 ? (
            <div className="text-center py-12 text-gray-500">暂无待审核项目</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {pendingProjects.map((project) => (
                <div key={project.id} className="p-6">
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-2">
                        <span className="px-2 py-1 bg-blue-100 text-blue-700 rounded text-xs font-medium">
                          {categoryMap[project.category]}
                        </span>
                        <span className="text-sm text-gray-500">
                          发布时间：{new Date(project.createdAt).toLocaleDateString()}
                        </span>
                      </div>
                      <h3 className="text-lg font-semibold text-gray-900 mb-2">{project.title}</h3>
                      <p className="text-gray-600 mb-2">{project.description}</p>
                      <div className="text-sm text-gray-500">
                        目标金额：¥{project.targetAmount.toLocaleString()}
                      </div>
                      <div className="text-sm text-gray-500">
                        执行计划：{project.executionPlan}
                      </div>
                    </div>
                    <div className="flex gap-2 ml-6">
                      <button
                        onClick={() => handleReviewProject(project.id, 'approved')}
                        className="px-4 py-2 bg-green-50 text-green-600 rounded-lg hover:bg-green-100"
                      >
                        通过
                      </button>
                      <button
                        onClick={() => handleReviewProject(project.id, 'rejected')}
                        className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100"
                      >
                        拒绝
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : activeTab === 'organizations' ? (
        <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
          {pendingOrgs.length === 0 ? (
            <div className="text-center py-12 text-gray-500">暂无待审核组织</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {pendingOrgs.map((org) => (
                <div key={org.id} className="p-6">
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <h3 className="text-lg font-semibold text-gray-900 mb-2">{org.name}</h3>
                      <p className="text-gray-600 mb-2">{org.description}</p>
                      <div className="grid grid-cols-2 gap-4 text-sm text-gray-500">
                        <div>执照编号：{org.licenseNumber || '-'}</div>
                        <div>联系人：{org.contactPerson || '-'}</div>
                        <div>联系电话：{org.contactPhone || '-'}</div>
                        <div>地址：{org.address || '-'}</div>
                      </div>
                    </div>
                    <div className="flex gap-2 ml-6">
                      <button
                        onClick={() => handleReviewOrg(org.id, 'approved')}
                        className="px-4 py-2 bg-green-50 text-green-600 rounded-lg hover:bg-green-100"
                      >
                        通过
                      </button>
                      <button
                        onClick={() => handleReviewOrg(org.id, 'rejected')}
                        className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100"
                      >
                        拒绝
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : (
        <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
          {pendingRefunds.length === 0 ? (
            <div className="text-center py-12 text-gray-500">暂无待处理退款申请</div>
          ) : (
            <div className="divide-y divide-gray-100">
              {pendingRefunds.map((refund) => (
                <div key={refund.id} className="p-6">
                  <div className="flex justify-between items-start">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-2">
                        <span className="px-2 py-1 bg-amber-100 text-amber-700 rounded text-xs font-medium">
                          待处理
                        </span>
                        <span className="text-sm text-gray-500">
                          申请时间：{new Date(refund.createdAt).toLocaleString()}
                        </span>
                      </div>
                      <h3 className="text-lg font-semibold text-gray-900 mb-1">
                        {refund.donation?.project?.title || `捐赠 #${refund.donationId}`}
                      </h3>
                      <div className="text-sm text-gray-500 mb-2">
                        捐赠人：{refund.donation?.user?.realName || refund.donation?.user?.username || '-'}
                      </div>
                      <div className="text-sm text-gray-600 mb-2">
                        退款原因：{refund.reason}
                      </div>
                      <div className="text-sm text-primary-600 font-semibold">
                        退款金额：¥{refund.donation?.amount.toLocaleString()}
                      </div>
                    </div>
                    <div className="flex gap-2 ml-6">
                      <button
                        onClick={() => handleReviewRefund(refund.id, 'approved')}
                        className="px-4 py-2 bg-green-50 text-green-600 rounded-lg hover:bg-green-100"
                      >
                        核准退款
                      </button>
                      <button
                        onClick={() => handleReviewRefund(refund.id, 'rejected')}
                        className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100"
                      >
                        驳回
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default Admin;
